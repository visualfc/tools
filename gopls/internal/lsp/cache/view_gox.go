// Copyright 2023 The GoPlus Authors (goplus.org). All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cache

import (
	"context"

	"golang.org/x/tools/gopls/internal/goxls/imports"
	"golang.org/x/tools/gopls/internal/span"
)

func (s *snapshot) GopRunProcessEnvFunc(ctx context.Context, fn func(context.Context, *imports.Options) error) error {
	return s.view.gopImportsState.runProcessEnvFunc(ctx, s, fn)
}

func gopAllFilesExcluded(goFiles, gopFiles []string, filterFunc func(span.URI) bool) bool {
	return allFilesExcluded(goFiles, filterFunc) && allFilesExcluded(gopFiles, filterFunc)
}
