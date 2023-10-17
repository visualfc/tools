// Copyright 2023 The GoPlus Authors (goplus.org). All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cache

import (
	"fmt"

	"golang.org/x/tools/gopls/internal/goxls/typesutil"
	"golang.org/x/tools/gopls/internal/lsp/source"
	"golang.org/x/tools/gopls/internal/span"
)

func (p *Package) GopTypesInfo() *typesutil.Info {
	return p.pkg.gopTypesInfo
}

func (p *Package) CompiledGopFiles() []*source.ParsedGopFile {
	return p.pkg.compiledGopFiles
}

func (p *Package) GopFile(uri span.URI) (*source.ParsedGopFile, error) {
	return p.pkg.GopFile(uri)
}

func (pkg *syntaxPackage) GopFile(uri span.URI) (*source.ParsedGopFile, error) {
	for _, cgf := range pkg.compiledGopFiles {
		if cgf.URI == uri {
			return cgf, nil
		}
	}
	for _, gf := range pkg.gopFiles {
		if gf.URI == uri {
			return gf, nil
		}
	}
	return nil, fmt.Errorf("no parsed file for %s in %v", uri, pkg.id)
}
