// Copyright 2023 The GoPlus Authors (goplus.org). All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package source

import (
	"context"

	"golang.org/x/tools/gopls/internal/goxls/parserutil"
	"golang.org/x/tools/gopls/internal/lsp/protocol"
	"golang.org/x/tools/internal/imports"
)

// GopAddImport adds a single import statement to the given file
func GopAddImport(ctx context.Context, snapshot Snapshot, fh FileHandle, importPath string) ([]protocol.TextEdit, error) {
	pgf, err := snapshot.ParseGop(ctx, fh, parserutil.ParseFull)
	if err != nil {
		return nil, err
	}
	return GopComputeOneImportFixEdits(snapshot, pgf, &imports.ImportFix{
		StmtInfo: imports.ImportInfo{
			ImportPath: importPath,
		},
		FixType: imports.AddImport,
	})
}
