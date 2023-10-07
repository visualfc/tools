// Copyright 2023 The GoPlus Authors (goplus.org). All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cache

import (
	"context"

	"github.com/goplus/gop/parser"
	"github.com/goplus/gop/token"
	"golang.org/x/tools/gopls/internal/lsp/source"
)

const (
	GopParseFull parser.Mode = parser.AllErrors | parser.ParseComments
)

// parseGopFiles returns a ParsedGopFile for each file handle in fhs, in the
// requested parse mode.
//
// For parsed files that already exists in the cache, access time will be
// updated. For others, parseFiles will parse and store as many results in the
// cache as space allows.
//
// The token.File for each resulting parsed file will be added to the provided
// FileSet, using the tokeninternal.AddExistingFiles API. Consequently, the
// given fset should only be used in other APIs if its base is >=
// reservedForParsing.
//
// If parseFiles returns an error, it still returns a slice,
// but with a nil entry for each file that could not be parsed.
func (c *parseCache) parseGopFiles(ctx context.Context, fset *token.FileSet, mode parser.Mode, purgeFuncBodies bool, fhs ...source.FileHandle) ([]*source.ParsedGopFile, error) {
	panic("todo")
}
