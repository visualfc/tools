// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The unmarshal command runs the unmarshal analyzer.
package main

import (
	"golang.org/x/tools/gop/analysis/passes/unmarshal"
	"golang.org/x/tools/gop/analysis/singlechecker"
)

func main() { singlechecker.Main(unmarshal.Analyzer) }
