// Copyright 2022 The GoPlus Authors (goplus.org). All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"reflect"
	"time"

	"golang.org/x/tools/internal/fakenet"
	"golang.org/x/tools/internal/jsonrpc2"
)

func main() {
	flag.Parse()

	app := "gopls"
	args := flag.Args()
	if len(args) > 0 {
		app = args[0]
	}

	fin, err := os.Open(app + ".in")
	check(err)
	defer fin.Close()

	fout, err := os.Open(app + ".out")
	check(err)
	defer fout.Close()

	reqStream := jsonrpc2.NewHeaderStream(fakenet.NewConn("request", fin, os.Stdout))
	respStream := jsonrpc2.NewHeaderStream(fakenet.NewConn("response", fout, os.Stdout))
	reqChan := make(chan jsonrpc2.ID, 1)
	respChan := make(chan *jsonrpc2.Response, 1)
	resps := make([]*jsonrpc2.Response, 0, 8)
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
				select {
				case <-time.After(time.Second):
				case resp := <-respChan:
					ret := any(resp.Err())
					if ret == nil {
						ret = params(resp.Result())
					} else {
						ret = fmt.Sprintf("%serror: %v\n", indent, ret)
					}
					log.Printf("[%v] ret:\n%s", id, ret)
				}
			case *jsonrpc2.Notification:
				log.Printf("[] %s:\n%s", req.Method(), params(req.Params()))
			}
		}
	}()
	go func() {
		ctx := context.Background()
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
	}()
	select {}
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
	return paramsEx(ret, indent)
}

func paramsEx(ret any, prefix string) []byte {
	var b bytes.Buffer
	switch val := ret.(type) {
	case mapt:
		for k, v := range val {
			if isComplex(v) {
				fmt.Fprintf(&b, "%s%s:\n%s", prefix, k, paramsEx(v, prefix+indent))
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

func check(err error) {
	if err != nil {
		log.Panicln(err)
	}
}
