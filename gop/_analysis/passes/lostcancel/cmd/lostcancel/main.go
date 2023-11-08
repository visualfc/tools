// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The lostcancel command applies the golang.org/x/tools/gop/analysis/passes/lostcancel
// analysis to the specified packages of Go source code.
package main

import (
	"golang.org/x/tools/gop/analysis/passes/lostcancel"
	"golang.org/x/tools/gop/analysis/singlechecker"
)

func main() { singlechecker.Main(lostcancel.Analyzer) }
