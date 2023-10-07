// Copyright 2023 The GoPlus Authors (goplus.org). All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cache

import (
	"context"

	"github.com/goplus/gop/parser"
	"golang.org/x/tools/gopls/internal/lsp/source"
)

// ParseGo parses the file whose contents are provided by fh, using a cache.
// The resulting tree may have beeen fixed up.
func (s *snapshot) ParseGop(ctx context.Context, fh source.FileHandle, mode parser.Mode) (*source.ParsedGopFile, error) {
	panic("todo")
}
