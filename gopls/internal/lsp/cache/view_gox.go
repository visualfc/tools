// Copyright 2023 The GoPlus Authors (goplus.org). All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cache

import (
	"context"

	"golang.org/x/tools/gopls/internal/goxls/imports"
)

func (s *snapshot) GopRunProcessEnvFunc(ctx context.Context, fn func(context.Context, *imports.Options) error) error {
	return s.view.gopImportsState.runProcessEnvFunc(ctx, s, fn)
}
