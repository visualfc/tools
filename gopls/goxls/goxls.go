// Copyright 2022 The GoPlus Authors (goplus.org). All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package goxls

import (
	"context"
	"io"
	"log"
	"os"
	"sync"

	"golang.org/x/tools/gopls/internal/goxls"
	"golang.org/x/tools/gopls/internal/hooks"
	"golang.org/x/tools/gopls/internal/lsp/cmd"
	"golang.org/x/tools/internal/event"
	"golang.org/x/tools/internal/event/core"
	"golang.org/x/tools/internal/event/export"
	"golang.org/x/tools/internal/event/label"
	"golang.org/x/tools/internal/tool"
)

const (
	FlagsRelease = 0
	FlagsDebug   = goxls.DbgFlagDefault | goxls.DbgFlagAnaAll
)

func Main(flags goxls.DbgFlags) {
	ctx := context.Background()
	goxls.SetDebug(flags)
	if os.Getenv("GOXLS_LOG_EVENT") != "" {
		var printer export.Printer
		var logw io.Writer
		var mutex sync.Mutex
		event.SetExporter(func(ctx context.Context, e core.Event, m label.Map) context.Context {
			mutex.Lock()
			defer mutex.Unlock()
			if logw == nil {
				logw = log.Writer()
			}
			printer.WriteEvent(logw, e, m)
			return ctx
		})
	}
	tool.Main(ctx, cmd.GopNew("goxls", "", nil, hooks.Options), os.Args[1:])
}
