// Copyright 2022 The GoPlus Authors (goplus.org). All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package lsview

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"reflect"
	"sort"
	"time"

	"golang.org/x/tools/internal/fakenet"
	"golang.org/x/tools/internal/jsonrpc2"
)

func Main(app, goxls string) {
	fin, err := os.Open(app + ".in")
	check(err)
	defer fin.Close()

	fdiff, err := os.Create(app + ".diff")
	check(err)
	defer fdiff.Close()

	logd := log.New(fdiff, "", log.LstdFlags)
	reqStream := jsonrpc2.NewHeaderStream(fakenet.NewConn("request", fin, os.Stdout))
	reqChan := make(chan jsonrpc2.ID, 1)
	respChan := make(chan *jsonrpc2.Response, 1)
	reqChan2 := make(chan jsonrpc2.ID, 1)
	respChan2 := make(chan *jsonrpc2.Response, 1)

	go func() {
		ctx := context.Background()
		for {
			msg, _, err := reqStream.Read(ctx)
			if err != nil {
				if errors.Is(err, io.EOF) {
					time.Sleep(time.Second / 5)
					continue
				}
				check(err)
			}
			switch req := msg.(type) {
			case *jsonrpc2.Call:
				id := req.ID()
				log.Printf("[%v] %s:\n%s", id, req.Method(), params(req.Params()))
				reqChan <- id
				resp, ret := respFetch(respChan)
				if resp != nil {
					log.Printf("[%v] %s ret:\n%s", id, app, resp)
				}
				if goxls != "" {
					select { // allow send request failed
					case <-time.After(time.Second):
					case reqChan2 <- id:
						if resp2, ret2 := respFetch(respChan2); resp2 != nil {
							log.Printf("[%v] %s ret:\n%s", id, goxls, resp2)
							if !reflect.DeepEqual(ret, ret2) {
								logd.Printf("[%v] %s:\n%s", id, req.Method(), params(req.Params()))
								logd.Printf("[%v] %s ret:\n%s", id, app, resp)
								logd.Printf("[%v] %s ret:\n%s", id, goxls, resp2)
							}
						}
					}
				}
			case *jsonrpc2.Notification:
				log.Printf("[] %s:\n%s", req.Method(), params(req.Params()))
			}
		}
	}()
	go respLoop(app, respChan, reqChan)
	if goxls != "" {
		go respLoop(app, respChan2, reqChan2)
	}
	select {}
}

func respFetch(respChan chan *jsonrpc2.Response) (any, any) {
	select {
	case <-time.After(time.Second):
	case resp := <-respChan:
		ret := any(resp.Err())
		if ret == nil {
			return paramsEx(resp.Result())
		}
		return fmt.Sprintf("%serror: %v\n", indent, ret), nil
	}
	return nil, nil
}

func respLoop(app string, respChan chan *jsonrpc2.Response, reqChan chan jsonrpc2.ID) {
	if app == "" {
		return
	}
	fout, err := os.Open(app + ".out")
	check(err)
	defer fout.Close()

	ctx := context.Background()
	respStream := jsonrpc2.NewHeaderStream(fakenet.NewConn("response", fout, os.Stdout))
	resps := make([]*jsonrpc2.Response, 0, 8)
next:
	id := <-reqChan
	for i, resp := range resps {
		if resp.ID() == id {
			resps = append(resps[i:], resps[i+1:]...)
			respChan <- resp
			goto next
		}
	}
	for {
		msg, _, err := respStream.Read(ctx)
		check(err)
		switch resp := msg.(type) {
		case *jsonrpc2.Response:
			if resp.ID() == id {
				respChan <- resp
				goto next
			}
			resps = append(resps, resp)
		}
	}
}

type any = interface{}
type mapt = map[string]any
type slice = []any

const indent = "  "

func params(raw json.RawMessage) []byte {
	var ret any
	err := json.Unmarshal(raw, &ret)
	if err != nil {
		return raw
	}
	return paramsFmt(ret, indent)
}

func paramsEx(raw json.RawMessage) ([]byte, any) {
	var ret any
	err := json.Unmarshal(raw, &ret)
	if err != nil {
		return raw, ret
	}
	return paramsFmt(ret, indent), ret
}

func paramsFmt(ret any, prefix string) []byte {
	var b bytes.Buffer
	switch val := ret.(type) {
	case mapt:
		keys := keys(val)
		for _, k := range keys {
			v := val[k]
			if isComplex(v) {
				fmt.Fprintf(&b, "%s%s:\n%s", prefix, k, paramsFmt(v, prefix+indent))
			} else {
				fmt.Fprintf(&b, "%s%s: %v\n", prefix, k, v)
			}
		}
	case slice:
		if isComplexSlice(val) {
			for _, v := range val {
				s, _ := json.Marshal(v)
				fmt.Fprintf(&b, "%s%s\n", prefix, s)
			}
		} else {
			fmt.Fprintf(&b, "%s%v\n", prefix, val)
		}
	default:
		log.Panicln("unexpected:", reflect.TypeOf(ret))
	}
	return b.Bytes()
}

func isComplexSlice(v slice) bool {
	if len(v) > 0 {
		return isComplex(v[0])
	}
	return false
}

func isComplex(v any) bool {
	if _, ok := v.(mapt); ok {
		return true
	}
	_, ok := v.(slice)
	return ok
}

func keys(v mapt) []string {
	ret := make([]string, 0, len(v))
	for key := range v {
		ret = append(ret, key)
	}
	sort.Strings(ret)
	return ret
}

func check(err error) {
	if err != nil {
		log.Panicln(err)
	}
}
