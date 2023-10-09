// Copyright 2023 The GoPlus Authors (goplus.org). All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package completion

import (
	"context"
	"errors"

	"github.com/goplus/gop/ast"
	"golang.org/x/tools/gopls/internal/span"
)

// packageNameCompletions returns name completions for a package clause using
// the current name as prefix.
func (c *gopCompleter) packageNameCompletions(ctx context.Context, fileURI span.URI, name *ast.Ident) error {
	cursor := int(c.pos - name.NamePos)
	if cursor < 0 || cursor > len(name.Name) {
		return errors.New("cursor is not in package name identifier")
	}

	c.completionContext.packageCompletion = true

	prefix := name.Name[:cursor]
	packageSuggestions, err := packageSuggestions(ctx, c.snapshot, fileURI, prefix)
	if err != nil {
		return err
	}

	for _, pkg := range packageSuggestions {
		c.deepState.enqueue(pkg)
	}
	return nil
}
