// Copyright 2023 The GoPlus Authors (goplus.org). All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Copyright 2023 The GoPlus Authors (goplus.org). All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cache

import (
	"golang.org/x/tools/gop/packages"
	"golang.org/x/tools/gopls/internal/lsp/source"
	"golang.org/x/tools/gopls/internal/span"
)

func fileNotSource(kind source.FileKind) bool {
	return kind != source.Go && kind != source.Gop
}

func packageIDSuffix(pkg *packages.Package) string {
	if len(pkg.CompiledGoFiles) > 0 {
		return pkg.CompiledGoFiles[0]
	}
	return pkg.CompiledGopFiles[0]
}

func collectSourceURIs(m *source.Metadata, in ...map[span.URI]struct{}) (uris map[span.URI]struct{}) {
	if in != nil && in[0] != nil {
		uris = in[0]
	} else {
		uris = map[span.URI]struct{}{}
	}
	// goxls: add Go+ files & use NongenGoFiles
	for _, uri := range m.CompiledNongenGoFiles {
		uris[uri] = struct{}{}
	}
	for _, uri := range m.GopFiles {
		uris[uri] = struct{}{}
	}
	for _, uri := range m.GoFiles {
		uris[uri] = struct{}{}
	}
	return uris
}
