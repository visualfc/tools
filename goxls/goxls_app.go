// Copyright 2023 The GoPlus Authors (goplus.org). All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// goxls is an LSP server for GoPlus.
// The Language Server Protocol allows any text editor
// to be extended with IDE-like features;
// see https://langserver.org/ for details.
//
// See https://github.com/goplus/tools/blob/goplus/goxls/README.md
// for the most up-to-date documentation.
package main // import "github.com/goplus/tools/goxls"

import (
	"golang.org/x/tools/gopls/goxls"
)

func main() {
	goxls.Main(nil, nil)
}
