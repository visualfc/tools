// Copyright 2023 The GoPlus Authors (goplus.org). All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"flag"
	"fmt"
)

// gopServe is a struct that exposes the configurable parts of the LSP server as
// flags, in the right form for tool.Main to consume.
type gopServe struct {
	*Serve
}

func newGopServe(app *GopApplication) *gopServe {
	return &gopServe{&app.Serve}
}

func (s *gopServe) ShortHelp() string {
	return "run a server for Go/Go+ code using the Language Server Protocol"
}
func (s *gopServe) DetailedHelp(f *flag.FlagSet) {
	fmt.Fprint(f.Output(), `  goxls [flags] [server-flags]

The server communicates using JSONRPC2 on stdin and stdout, and is intended to be run directly as
a child of an editor process.

server-flags:
`)
	printFlagDefaults(f)
}
