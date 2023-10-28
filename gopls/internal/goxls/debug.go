// Copyright 2023 The GoPlus Authors (goplus.org). All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package goxls

import (
	"github.com/goplus/gop/cl"
	"github.com/goplus/gop/x/typesutil"
)

type dbgFlags int

const (
	DbgFlagTypesUtil = 1 << iota
	DbgFlagDisableRecover
	DbgFlagCompletion
	DbgFlagCodeAction
	DbgFlagHover
	DbgFlagHighlight
	DbgFlagDefinition
	DbgFlagCommand
	DbgFlagCodeLens
	DbgFlagDefault = DbgFlagTypesUtil | DbgFlagCompletion | DbgFlagCodeAction | DbgFlagHover |
		DbgFlagHighlight | DbgFlagDefinition | DbgFlagCommand | DbgFlagCodeLens
	DbgFlagAll = DbgFlagDefault | DbgFlagDisableRecover
)

const (
	DbgMisuse = true
)

var (
	DbgCompletion bool
	DbgCodeAction bool
	DbgHover      bool
	DbgHighlight  bool
	DbgDefinition bool
	DbgCommand    bool
	DbgCodeLens   bool
)

func SetDebug(flags dbgFlags) {
	if (flags & DbgFlagTypesUtil) != 0 {
		typesutil.SetDebug(typesutil.DbgFlagDefault)
	}
	if (flags & DbgFlagDisableRecover) != 0 {
		cl.SetDisableRecover(true)
	}
	DbgCompletion = (flags & DbgFlagCompletion) != 0
	DbgHover = (flags & DbgFlagHover) != 0
	DbgCodeAction = (flags & DbgFlagCodeAction) != 0
	DbgHighlight = (flags & DbgFlagHighlight) != 0
	DbgDefinition = (flags & DbgFlagDefinition) != 0
	DbgCommand = (flags & DbgFlagCommand) != 0
	DbgCodeLens = (flags & DbgFlagCodeLens) != 0
}
