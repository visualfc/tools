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
	DbgFlagCompletion
	DbgFlagDisableRecover
	DbgFlagDefault = DbgFlagTypesUtil | DbgFlagCompletion
	DbgFlagAll     = DbgFlagDefault | DbgFlagDisableRecover
)

var (
	DbgCompletion bool
)

func SetDebug(flags dbgFlags) {
	if (flags & DbgFlagTypesUtil) != 0 {
		typesutil.SetDebug(typesutil.DbgFlagDefault)
	}
	if (flags & DbgFlagDisableRecover) != 0 {
		cl.SetDisableRecover(true)
	}
	DbgCompletion = (flags & DbgFlagCompletion) != 0
}
