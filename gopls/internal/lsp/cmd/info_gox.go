// Copyright 2023 The GoPlus Authors (goplus.org). All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"context"
	"flag"
	"strings"

	"golang.org/x/tools/internal/tool"
)

// gopHelp implements the help command.
type gopHelp struct {
	help
	app *GopApplication
}

func newGopHelp(app *GopApplication) *gopHelp {
	return &gopHelp{
		help: help{app: &app.Application},
		app:  app,
	}
}

// Run prints help information about a subcommand.
func (h *gopHelp) Run(ctx context.Context, args ...string) error {
	find := func(cmds []tool.Application, name string) tool.Application {
		for _, cmd := range cmds {
			if cmd.Name() == name {
				return cmd
			}
		}
		return nil
	}

	// Find the subcommand denoted by args (empty => h.app).
	var cmd tool.Application = h.app
	for i, arg := range args {
		cmd = find(getSubcommands(cmd), arg)
		if cmd == nil {
			return tool.CommandLineErrorf(
				"no such subcommand: %s", strings.Join(args[:i+1], " "))
		}
	}

	// 'gopls help cmd subcmd' is equivalent to 'gopls cmd subcmd -h'.
	// The flag package prints the usage information (defined by tool.Run)
	// when it sees the -h flag.
	fs := flag.NewFlagSet(cmd.Name(), flag.ExitOnError)
	return tool.Run(ctx, fs, h.app, append(args[:len(args):len(args)], "-h"))
}

// gopVersion implements the version command.
type gopVersion struct {
	version
	app *GopApplication
}

func newGopVersion(app *GopApplication) *gopVersion {
	return &gopVersion{
		version: version{app: &app.Application},
		app:     app,
	}
}

func (v *gopVersion) ShortHelp() string { return "print the goxls version information" }

// gopBug implements the bug command.
type gopBug struct {
	bug
	app *GopApplication
}

func newGopBug(app *GopApplication) *gopBug {
	return &gopBug{
		bug: bug{app: &app.Application},
		app: app,
	}
}

func (b *gopBug) ShortHelp() string { return "report a bug in goxls" }

type gopApiJSON struct {
	apiJSON
	app *GopApplication
}

func newGopApiJSON(app *GopApplication) *gopApiJSON {
	return &gopApiJSON{
		apiJSON: apiJSON{app: &app.Application},
		app:     app,
	}
}

func (j *gopApiJSON) ShortHelp() string { return "print json describing goxls API" }
