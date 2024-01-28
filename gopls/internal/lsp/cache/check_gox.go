// Copyright 2023 The GoPlus Authors (goplus.org). All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cache

import (
	goast "go/ast"
	"go/types"

	"github.com/goplus/gop/ast"
	"github.com/goplus/gop/x/typesutil"
	"golang.org/x/tools/gopls/internal/lsp/source"
)

func newGopTypeInfo() *typesutil.Info {
	return &typesutil.Info{
		Types:      make(map[ast.Expr]types.TypeAndValue),
		Defs:       make(map[*ast.Ident]types.Object),
		Uses:       make(map[*ast.Ident]types.Object),
		Implicits:  make(map[ast.Node]types.Object),
		Selections: make(map[*ast.SelectorExpr]*types.Selection),
		Scopes:     make(map[ast.Node]*types.Scope),
		Instances:  make(map[*ast.Ident]types.Instance),
	}
}

func checkCompiledFiles(check *typesutil.Checker, goFiles []*goast.File, compiledGopFiles []*source.ParsedGopFile) error {
	gopFiles := make([]*ast.File, 0, len(compiledGopFiles))
	for _, cgf := range compiledGopFiles {
		gopFiles = append(gopFiles, cgf.File)
	}
	return check.Files(goFiles, gopFiles)
}
