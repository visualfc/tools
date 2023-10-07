// Copyright 2023 The GoPlus Authors (goplus.org). All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"context"

	"golang.org/x/tools/gopls/internal/lsp/cmd"
	"golang.org/x/tools/gopls/internal/lsp/source"
)

// Application is the main application as passed to tool.Main
// It handles the main command line parsing and dispatch to the sub commands.
type Application struct {
	*cmd.Application
}

// New returns a new Application ready to run.
func New(name, wd string, env []string, options func(*source.Options)) Application {
	app := cmd.New(name, wd, env, options)
	return Application{app}
}

// Run takes the args after top level flag processing, and invokes the correct
// sub command as specified by the first argument.
// If no arguments are passed it will invoke the server sub command, as a
// temporary measure for compatibility.
func (app Application) Run(ctx context.Context, args ...string) error {
	return app.Application.Run(ctx, args...)
}
