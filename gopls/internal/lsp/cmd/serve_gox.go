// Copyright 2023 The GoPlus Authors (goplus.org). All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"

	qlog "github.com/qiniu/x/log"
	"golang.org/x/tools/gop/langserver"
)

// gopServe is a struct that exposes the configurable parts of the LSP server as
// flags, in the right form for tool.Main to consume.
type gopServe struct {
	*Serve
}

func newGopServe(app *Application) gopServe {
	return gopServe{&app.Serve}
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

// Run configures a server based on the flags, and then runs it.
// It blocks until the server shuts down.
func (s *gopServe) Run(ctx context.Context, args ...string) error {
	if os.Getenv("GOXLS_LOG_FILE") != "" {
		if home, err := os.UserHomeDir(); err == nil {
			goxlsDir := home + "/.goxls"
			os.MkdirAll(goxlsDir, 0755)
			if f, err := os.Create(goxlsDir + "/serve.log"); err == nil {
				defer f.Close()
				w := io.MultiWriter(f, os.Stderr)
				log.SetOutput(w)
				qlog.SetOutput(w)
			}
		}
	}

	langserver.Initialize()
	defer langserver.Shutdown()

	return s.Serve.Run(ctx, args...)
}
