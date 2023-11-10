// Copyright 2023 The GoPlus Authors (goplus.org). All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

type RemoteBase = remote

type gopRemote struct {
	RemoteBase
	app *GopApplication
}

func newGopRemote(app *GopApplication, alias string) *gopRemote {
	remote := newRemote(&app.Application, alias)
	return &gopRemote{RemoteBase: *remote, app: app}
}

func (r *gopRemote) ShortHelp() string {
	short := "interact with the goxls daemon"
	if r.alias != "" {
		short += " (deprecated: use 'remote')"
	}
	return short
}
