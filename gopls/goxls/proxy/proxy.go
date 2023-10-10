// Copyright 2022 The GoPlus Authors (goplus.org). All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package proxy

import (
	"io"
	"log"
	"os"
	"os/exec"
	"strconv"
	"time"
)

const (
	gopls = "gopls.origin"
)

func Main() {
	home, err := os.UserHomeDir()
	check(err)

	goplsDir := home + "/.gopls"
	rotateDir := goplsDir + "/" + strconv.FormatInt(time.Now().UnixMicro(), 36)
	err = os.MkdirAll(rotateDir, 0755)
	check(err)

	createFile := func(name string) (f *os.File, err error) {
		normal, rotate := goplsDir+name, rotateDir+name
		os.Rename(normal, rotate)
		return os.Create(normal)
	}

	logf, err := createFile("/gopls.log")
	check(err)
	defer logf.Close()

	stdinf, err := createFile("/gopls.in")
	check(err)
	defer stdinf.Close()

	stdoutf, err := createFile("/gopls.out")
	check(err)
	defer stdoutf.Close()

	stderrf, err := createFile("/gopls.err")
	check(err)
	defer stderrf.Close()

	log.SetOutput(logf)
	log.Println("[INFO] app start:", os.Args)

	pr, pw, err := os.Pipe()
	check(err)
	go func() {
		w := io.MultiWriter(pw, stdinf)
		for {
			io.Copy(w, os.Stdin)
		}
	}()

	cmd := exec.Command(gopls, os.Args[1:]...)
	cmd.Stdin = pr
	cmd.Stdout = io.MultiWriter(os.Stdout, stdoutf)
	cmd.Stderr = io.MultiWriter(os.Stderr, stderrf)
	err = cmd.Run()
	check(err)
}

func check(err error) {
	if err != nil {
		log.Panicln(err)
	}
}
