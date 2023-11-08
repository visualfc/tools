// Copyright 2023 The GoPlus Authors (goplus.org). All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package parserutil

import (
	"path/filepath"

	"github.com/goplus/gop/ast"
	"github.com/goplus/gop/parser"
	"github.com/goplus/gop/token"
	"golang.org/x/tools/gopls/internal/goxls/goputil"
)

const (
	// ParseHeader specifies that the main package declaration and imports are needed.
	// This is the mode used when attempting to examine the package graph structure.
	ParseHeader = parser.AllErrors | parser.ParseComments | parser.ImportsOnly

	// ParseFull specifies the full AST is needed.
	// This is used for files of direct interest where the entire contents must
	// be considered.
	ParseFull = parser.AllErrors | parser.ParseComments
)

func ParseFile(fset *token.FileSet, filename string, src interface{}, mode parser.Mode) (f *ast.File, err error) {
	var isClass bool
	if goputil.FileKind(filepath.Ext(filename)) == goputil.FileGopClass {
		isClass = true
		mode |= parser.ParseGoPlusClass
	}
	f, err = parser.ParseFile(fset, filename, src, mode)
	f.IsClass = isClass
	return
}
