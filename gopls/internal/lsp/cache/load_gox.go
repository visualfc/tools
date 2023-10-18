// Copyright 2023 The GoPlus Authors (goplus.org). All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Copyright 2023 The GoPlus Authors (goplus.org). All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cache

import (
	"golang.org/x/tools/gopls/internal/goxls/packages"
	"golang.org/x/tools/gopls/internal/lsp/source"
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
