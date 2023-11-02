// Copyright 2023 The GoPlus Authors (goplus.org). All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cache

import (
	goast "go/ast"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"strings"

	"github.com/goplus/gop/ast"
	"github.com/goplus/gox"
	"github.com/qiniu/x/errors"
	"golang.org/x/tools/gopls/internal/goxls/typesutil"
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
	}
}

func checkFiles(check *typesutil.Checker, goFiles []*goast.File, compiledGopFiles []*source.ParsedGopFile) error {
	gopFiles := make([]*ast.File, 0, len(compiledGopFiles))
	for _, cgf := range compiledGopFiles {
		gopFiles = append(gopFiles, cgf.File)
	}
	return check.Files(goFiles, gopFiles)
}

// extractGopTypeErrors extracts GopTypeError list from gop type check result
func extractGopTypeErrors(checkErr error, pkg *syntaxPackage) (gopTypeErrs []typesutil.GopTypeError) {
	gopfs := make(map[string]*token.File)
	pkg.fset.Iterate(func(f *token.File) bool {
		workingDir, _ := os.Getwd()
		fname := relFile(workingDir, f.Name())
		gopfs[fname] = f
		return true
	})

	extractErr := func(err error) {
		switch e := err.(type) {
		case *gox.CodeError:
			pos := token.Pos(0)
			if f, ok := gopfs[e.Pos.Filename]; ok {
				pos = token.Pos(f.Base() + e.Pos.Offset)
			}
			gopTypeErrs = append(gopTypeErrs, typesutil.GopTypeError{
				Fset: pkg.fset,
				Pos:  pos,
				Msg:  e.Msg,
			})
		case *gox.MatchError:
			gopTypeErrs = append(gopTypeErrs, typesutil.GopTypeError{
				Fset: pkg.fset,
				Pos:  e.Src.Pos(),
				Msg:  matchErrMsg(e),
			})
		case *gox.ImportError:
			pos := token.Pos(0)
			if f, ok := gopfs[e.Pos.Filename]; ok {
				pos = token.Pos(f.Base() + e.Pos.Offset)
			}
			gopTypeErrs = append(gopTypeErrs, typesutil.GopTypeError{
				Fset: pkg.fset,
				Pos:  pos,
				Msg:  e.Unwrap().Error(),
			})
		}
	}
	if errList, ok := checkErr.(errors.List); ok {
		for _, e := range errList {
			extractErr(e)
		}
	} else {
		extractErr(checkErr)
	}
	return
}

func matchErrMsg(err *gox.MatchError) string {
	msg := err.Error()
	// remove position info at the head
	colonIdx := strings.Index(msg, ": ")
	if colonIdx >= 0 {
		msg = msg[colonIdx+2:]
	}
	return msg
}

// relFile from github.com/goplus/gop/blob/main/cl/stmt.go
func relFile(dir string, file string) string {
	if rel, err := filepath.Rel(dir, file); err == nil {
		if rel[0] == '.' {
			return rel
		}
		return "./" + rel
	}
	return file
}
