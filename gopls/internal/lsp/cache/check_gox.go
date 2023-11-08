// Copyright 2023 The GoPlus Authors (goplus.org). All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cache

import (
	"fmt"
	goast "go/ast"
	"go/types"
	"log"
	"path"
	"path/filepath"

	"github.com/goplus/gop/ast"
	"github.com/goplus/mod/gopmod"
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

func checkFiles(mod *gopmod.Module, check *typesutil.Checker, goFiles []*goast.File, compiledGopFiles []*source.ParsedGopFile) error {
	var classKind func(fname string) (isProj bool, ok bool)
	if mod != nil {
		classKind = mod.ClassKind
	} else {
		classKind = defaultClassKind
	}
	gopFiles := make([]*ast.File, 0, len(compiledGopFiles))
	for _, cgf := range compiledGopFiles {
		err := checkFileClass(classKind, cgf.File, cgf.URI.Filename())
		if err != nil {
			// TODO error
			log.Println(err)
		}
		gopFiles = append(gopFiles, cgf.File)
	}
	return check.Files(goFiles, gopFiles)
}

func defaultClassKind(fname string) (isProj bool, ok bool) {
	ext := path.Ext(fname)
	switch ext {
	case ".gmx":
		return true, true
	case ".spx":
		return fname == "main.spx", true
	}
	return
}

func checkFileClass(classKind func(fname string) (isProj bool, ok bool), f *ast.File, fname string) error {
	var isClass, isProj, isNormalGox, rmGox bool
	fnameRmGox := fname
	ext := filepath.Ext(fname)
	switch ext {
	case ".gop":
		return nil
	case ".gox":
		isClass = true
		t := fname[:len(fname)-4]
		if c := filepath.Ext(t); c != "" {
			ext, fnameRmGox, rmGox = c, t, true
		} else {
			isNormalGox = true
		}
		fallthrough
	default:
		if !isNormalGox {
			if isProj, isClass = classKind(fnameRmGox); !isClass {
				if rmGox {
					return fmt.Errorf("not found Go+ class by ext %q for %q", ext, fname)
				}
			}
		}
	}
	f.IsClass = isClass
	f.IsProj = isProj
	f.IsNormalGox = isNormalGox
	return nil
}
