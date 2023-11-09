// Copyright 2022 The GoPlus Authors (goplus.org). All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package goxls

import (
	"context"
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

func Main() {
	ctx := context.Background()
	var printer export.Printer
	var mutex sync.Mutex
	event.SetExporter(func(ctx context.Context, e core.Event, m label.Map) context.Context {
		mutex.Lock()
		defer mutex.Unlock()
		printer.WriteEvent(os.Stderr, e, m)
		return ctx
	})
	goxls.SetDebug(goxls.DbgFlagDefault)
	tool.Main(ctx, cmd.GopNew("goxls", "", nil, hooks.Options), os.Args[1:])
}
