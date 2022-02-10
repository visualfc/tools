// Copyright 2022 The GoPlus Authors (goplus.org). All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// Goplsp is an LSP server for GoPlus.
// The Language Server Protocol allows any text editor
// to be extended with IDE-like features;
// see https://langserver.org/ for details.
//
// See https://github.com/goplus/tools/blob/goplus/goplsp/README.md
// for the most up-to-date documentation.
package main // import "github.com/goplus/tools/goplsp"

import (
	"golang.org/x/tools/gopls/goplsp"
)

func main() {
	goplsp.Main()
}
