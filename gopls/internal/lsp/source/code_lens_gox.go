// Copyright 2023 The GoPlus Authors (goplus.org). All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package source

import (
	"context"
	"go/types"
	"regexp"
	"strings"

	"github.com/goplus/gop/ast"
)

func GopTestsAndBenchmarks(ctx context.Context, snapshot Snapshot, pkg Package, pgf *ParsedGopFile) (testFns, error) {
	var out testFns

	if !strings.HasSuffix(pgf.URI.Filename(), "_test.go") {
		return out, nil
	}

	for _, d := range pgf.File.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if !ok {
			continue
		}

		rng, err := pgf.NodeRange(fn)
		if err != nil {
			return out, err
		}

		if gopMatchTestFunc(fn, pkg, testRe, "T") {
			out.Tests = append(out.Tests, testFn{fn.Name.Name, rng})
		}

		if gopMatchTestFunc(fn, pkg, benchmarkRe, "B") {
			out.Benchmarks = append(out.Benchmarks, testFn{fn.Name.Name, rng})
		}
	}

	return out, nil
}

func gopMatchTestFunc(fn *ast.FuncDecl, pkg Package, nameRe *regexp.Regexp, paramID string) bool {
	// Make sure that the function name matches a test function.
	if !nameRe.MatchString(fn.Name.Name) {
		return false
	}
	info := pkg.GopTypesInfo()
	if info == nil {
		return false
	}
	obj := info.ObjectOf(fn.Name)
	if obj == nil {
		return false
	}
	sig, ok := obj.Type().(*types.Signature)
	if !ok {
		return false
	}
	// Test functions should have only one parameter.
	if sig.Params().Len() != 1 {
		return false
	}

	// Check the type of the only parameter
	paramTyp, ok := sig.Params().At(0).Type().(*types.Pointer)
	if !ok {
		return false
	}
	named, ok := paramTyp.Elem().(*types.Named)
	if !ok {
		return false
	}
	namedObj := named.Obj()
	if namedObj.Pkg().Path() != "testing" {
		return false
	}
	return namedObj.Id() == paramID
}
