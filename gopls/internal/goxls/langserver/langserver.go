// Copyright 2023 The GoPlus Authors (goplus.org). All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package langserver

import (
	"context"
	"sync"

	"github.com/goplus/gop/x/langserver"
)

const (
	Enable = true
)

var (
	ls langserver.Client
)

func Initialize() {
	if Enable {
		go Get()
	}
}

func Shutdown() {
	if Enable {
		Get().Close()
	}
}

var (
	onceInit sync.Once
)

func Get() langserver.Client {
	onceInit.Do(func() {
		ls = langserver.ServeAndDial(nil, "gop", "serve", "-v")
	})
	return ls
}

func GenGo(ctx context.Context, pattern ...string) error {
	if Enable {
		return Get().GenGo(ctx, pattern...)
	}
	return langserver.GenGo(pattern...)
}

func Changed(ctx context.Context, files ...string) error {
	if Enable {
		return Get().Changed(ctx, files...)
	}
	return nil
}
