// Copyright 2023 The GoPlus Authors (goplus.org). All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package source

import (
	"fmt"

	"github.com/goplus/gop/ast"
	"github.com/goplus/gop/token"
	"golang.org/x/tools/gopls/internal/goxls/astutil"
)

// GopCanInvertIfCondition reports whether we can do invert-if-condition on the
// code in the given range
func GopCanInvertIfCondition(file *ast.File, start, end token.Pos) (*ast.IfStmt, bool, error) {
	path, _ := astutil.PathEnclosingInterval(file, start, end)
	for _, node := range path {
		stmt, isIfStatement := node.(*ast.IfStmt)
		if !isIfStatement {
			continue
		}

		if stmt.Else == nil {
			// Can't invert conditions without else clauses
			return nil, false, fmt.Errorf("else clause required")
		}

		if _, hasElseIf := stmt.Else.(*ast.IfStmt); hasElseIf {
			// Can't invert conditions with else-if clauses, unclear what that
			// would look like
			return nil, false, fmt.Errorf("else-if not supported")
		}

		return stmt, true, nil
	}

	return nil, false, fmt.Errorf("not an if statement")
}
