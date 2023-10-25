// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package source

import (
	"context"
	"log"

	"golang.org/x/tools/gopls/internal/goxls"
	"golang.org/x/tools/gopls/internal/lsp/protocol"
	"golang.org/x/tools/internal/imports"
)

// AddImport adds a single import statement to the given file
func AddImport(ctx context.Context, snapshot Snapshot, fh FileHandle, importPath string) ([]protocol.TextEdit, error) {
	// goxls: Go+
	kind := snapshot.View().FileKind(fh)
	if goxls.DbgCommand {
		log.Println("commandHandler.AddImport:", kind)
	}
	if kind == Gop {
		return GopAddImport(ctx, snapshot, fh, importPath)
	}

	pgf, err := snapshot.ParseGo(ctx, fh, ParseFull)
	if err != nil {
		return nil, err
	}
	return ComputeOneImportFixEdits(snapshot, pgf, &imports.ImportFix{
		StmtInfo: imports.ImportInfo{
			ImportPath: importPath,
		},
		FixType: imports.AddImport,
	})
}
