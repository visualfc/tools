// Copyright 2023 The GoPlus Authors (goplus.org). All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package lsp

import (
	"context"
	"sort"

	"golang.org/x/tools/gopls/internal/goxls/astutil"
	"golang.org/x/tools/gopls/internal/goxls/parserutil"
	"golang.org/x/tools/gopls/internal/lsp/command"
	"golang.org/x/tools/gopls/internal/lsp/source"
	"golang.org/x/tools/internal/tokeninternal"
)

func gopListImportsCmd(result *command.ListImportsResult, ctx context.Context, args command.URIArg, deps commandDeps) error {
	fh := deps.fh
	pgf, err := deps.snapshot.ParseGop(ctx, fh, parserutil.ParseHeader)
	if err != nil {
		return err
	}
	fset := tokeninternal.FileSetFor(pgf.Tok)
	for _, group := range astutil.Imports(fset, pgf.File) {
		for _, imp := range group {
			if imp.Path == nil {
				continue
			}
			var name string
			if imp.Name != nil {
				name = imp.Name.Name
			}
			result.Imports = append(result.Imports, command.FileImport{
				Path: string(source.GopUnquoteImportPath(imp)),
				Name: name,
			})
		}
	}
	meta, err := source.NarrowestMetadataForFile(ctx, deps.snapshot, args.URI.SpanURI())
	if err != nil {
		return err // e.g. cancelled
	}
	for pkgPath := range meta.DepsByPkgPath {
		result.PackageImports = append(result.PackageImports,
			command.PackageImport{Path: string(pkgPath)})
	}
	sort.Slice(result.PackageImports, func(i, j int) bool {
		return result.PackageImports[i].Path < result.PackageImports[j].Path
	})
	return nil
}
