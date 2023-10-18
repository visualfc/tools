// Copyright 2023 The GoPlus Authors (goplus.org). All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fillstruct

import (
	"fmt"
	"go/types"
	"strings"

	"github.com/goplus/gop/ast"
	"github.com/goplus/gop/token"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/gopls/internal/goxls/inspector"
	"golang.org/x/tools/gopls/internal/goxls/typesutil"
)

// GopDiagnoseFillableStructs computes diagnostics for fillable struct composite
// literals overlapping with the provided start and end position.
//
// If either start or end is invalid, it is considered an unbounded condition.
func GopDiagnoseFillableStructs(inspect *inspector.Inspector, start, end token.Pos, pkg *types.Package, info *typesutil.Info) []analysis.Diagnostic {
	var diags []analysis.Diagnostic
	nodeFilter := []ast.Node{(*ast.CompositeLit)(nil)}
	inspect.Preorder(nodeFilter, func(n ast.Node) {
		expr := n.(*ast.CompositeLit)

		if (start.IsValid() && expr.End() < start) || (end.IsValid() && expr.Pos() > end) {
			return // non-overlapping
		}

		typ := info.TypeOf(expr)
		if typ == nil {
			return
		}

		// Find reference to the type declaration of the struct being initialized.
		typ = deref(typ)
		tStruct, ok := typ.Underlying().(*types.Struct)
		if !ok {
			return
		}
		// Inv: typ is the possibly-named struct type.

		fieldCount := tStruct.NumFields()

		// Skip any struct that is already populated or that has no fields.
		if fieldCount == 0 || fieldCount == len(expr.Elts) {
			return
		}

		// Are any fields in need of filling?
		var fillableFields []string
		for i := 0; i < fieldCount; i++ {
			field := tStruct.Field(i)
			// Ignore fields that are not accessible in the current package.
			if field.Pkg() != nil && field.Pkg() != pkg && !field.Exported() {
				continue
			}
			fillableFields = append(fillableFields, fmt.Sprintf("%s: %s", field.Name(), field.Type().String()))
		}
		if len(fillableFields) == 0 {
			return
		}

		// Derive a name for the struct type.
		var name string
		if typ != tStruct {
			// named struct type (e.g. pkg.S[T])
			name = types.TypeString(typ, types.RelativeTo(pkg))
		} else {
			// anonymous struct type
			totalFields := len(fillableFields)
			const maxLen = 20
			// Find the index to cut off printing of fields.
			var i, fieldLen int
			for i = range fillableFields {
				if fieldLen > maxLen {
					break
				}
				fieldLen += len(fillableFields[i])
			}
			fillableFields = fillableFields[:i]
			if i < totalFields {
				fillableFields = append(fillableFields, "...")
			}
			name = fmt.Sprintf("anonymous struct { %s }", strings.Join(fillableFields, ", "))
		}
		diags = append(diags, analysis.Diagnostic{
			Message: fmt.Sprintf("Fill %s", name),
			Pos:     expr.Pos(),
			End:     expr.End(),
		})
	})

	return diags
}
