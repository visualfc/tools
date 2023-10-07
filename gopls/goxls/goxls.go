// Copyright 2022 The GoPlus Authors (goplus.org). All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package goxls

import (
	"context"
	"os"

	"golang.org/x/tools/gopls/internal/hooks"
	"golang.org/x/tools/gopls/internal/lsp/cmd"
	"golang.org/x/tools/gopls/internal/telemetry"
	"golang.org/x/tools/internal/tool"
)

func Main() {
	telemetry.Start()
	ctx := context.Background()
	tool.Main(ctx, cmd.New("goxls", "", nil, hooks.Options), os.Args[1:])
}
