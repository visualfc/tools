// Copyright 2023 The GoPlus Authors (goplus.org). All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package typesutil

import (
	"bytes"

	"github.com/goplus/gop/ast"
	"github.com/goplus/gop/x/typesutil"
)

// ExprString returns the (possibly shortened) string representation for x.
// Shortened representations are suitable for user interfaces but may not
// necessarily follow Go syntax.
func ExprString(x ast.Expr) string {
	return typesutil.ExprString(x)
}

// WriteExpr writes the (possibly shortened) string representation for x to buf.
// Shortened representations are suitable for user interfaces but may not
// necessarily follow Go syntax.
func WriteExpr(buf *bytes.Buffer, x ast.Expr) {
	typesutil.WriteExpr(buf, x)
}
