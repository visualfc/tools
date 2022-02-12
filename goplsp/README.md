# `goplsp`, the Go+ language server

`goplsp` is the official Go+ [language server] developed
by the Go+ team. It provides IDE features to any [LSP]-compatible editor.

## Editors

To get started with `goplsp`, install an LSP plugin in your editor of choice.

* TODO

[language server]: https://langserver.org
[LSP]: https://microsoft.github.io/language-server-protocol/

## How to release `goplsp`

* Create branch `release-branch.vX.Y.ZZ` from branch `goplus`.
* Tag branch `release-branch.vX.Y.ZZ` with `vX.Y.ZZ`.
* Update `gopls/go.mod` with `replace golang.org/x/tools => github.com/goplus/tools vX.Y.ZZ`.
* Execute `go mod tidy` in `gopls` module.
* Tag branch `release-branch.vX.Y.ZZ` with `gopls/vX.Y.ZZ`.
* Update `goplsp/go.mod` with `replace golang.org/x/tools => github.com/goplus/tools vX.Y.ZZ` and `replace golang.org/x/tools/gopls => github.com/goplus/tools/gopls vX.Y.ZZ`.
* Execute `go mod tidy` in `goplsp` module.
* Tag branch `release-branch.vX.Y.ZZ` with `goplsp/vX.Y.ZZ`.
