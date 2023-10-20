// Copyright 2023 The GoPlus Authors (goplus.org). All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package typesutil

import (
	"go/types"

	"github.com/goplus/gop/token"
	"github.com/goplus/gop/x/c2go"
	"github.com/goplus/gop/x/typesutil"
	"github.com/goplus/mod/gopmod"
)

// A Checker maintains the state of the type checker.
// It must be created with NewChecker.
type Checker = typesutil.Checker

type Config struct {
	// Types provides type information for the package (optional).
	Types *types.Package

	// Fset provides source position information for syntax trees and types (required).
	// If Fset is nil, Load will use a new fileset, but preserve Fset's value.
	Fset *token.FileSet

	// Mod represents a gop.mod object.
	Mod *gopmod.Module
}

// NewChecker returns a new Checker instance for a given package.
// Package files may be added incrementally via checker.Files.
func NewChecker(conf *types.Config, opts *Config, goInfo *types.Info, gopInfo *Info) *Checker {
	mod := opts.Mod
	if mod == nil {
		mod = gopmod.Default
	}
	chkOpts := &typesutil.Config{
		Types:       opts.Types,
		Fset:        opts.Fset,
		LookupPub:   c2go.LookupPub(mod),
		LookupClass: mod.LookupClass,
	}
	return typesutil.NewChecker(conf, chkOpts, goInfo, gopInfo)
}

func init() {
	typesutil.SetDebug(typesutil.DbgFlagDefault)
}
