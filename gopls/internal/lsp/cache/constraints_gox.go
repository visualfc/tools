// Copyright 2023 The GoPlus Authors (goplus.org). All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cache

import (
	"golang.org/x/tools/gopls/internal/lsp/source"
)

// isStandaloneFileEx reports whether a file with the given contents should be
// considered a 'standalone main file', meaning a package that consists of only
// a single file.
func isStandaloneFileEx(kind source.FileKind, src []byte, standaloneTags []string) bool {
	if kind == source.Gop {
		return false
	}
	return isStandaloneFile(src, standaloneTags)
}
