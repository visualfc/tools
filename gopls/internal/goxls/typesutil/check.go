// Copyright 2023 The GoPlus Authors (goplus.org). All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package typesutil

import (
	goast "go/ast"
	"go/types"

	"github.com/goplus/gop/ast"
	"github.com/goplus/gop/token"
)

// A Checker maintains the state of the type checker.
// It must be created with NewChecker.
type Checker struct {
	c *types.Checker
}

// NewChecker returns a new Checker instance for a given package.
// Package files may be added incrementally via checker.Files.
func NewChecker(
	conf *types.Config, fset *token.FileSet, pkg *types.Package,
	goInfo *types.Info, gopInfo *Info) *Checker {
	check := types.NewChecker(conf, fset, pkg, goInfo)
	return &Checker{check}
}

// Files checks the provided files as part of the checker's package.
func (p *Checker) Files(goFiles []*goast.File, gopFiles []*ast.File) error {
	if len(gopFiles) == 0 {
		return p.c.Files(goFiles)
	}
	// goxls: todo
	return nil
}
