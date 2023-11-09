// Copyright 2023 The GoPlus Authors (goplus.org). All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"context"
	"flag"
	"fmt"
	"text/tabwriter"

	"golang.org/x/tools/gopls/internal/lsp/debug"
	"golang.org/x/tools/gopls/internal/lsp/filecache"
	"golang.org/x/tools/gopls/internal/lsp/source"
	"golang.org/x/tools/internal/tool"
)

// GopApplication is the main application as passed to tool.Main
// It handles the main command line parsing and dispatch to the sub commands.
type GopApplication struct {
	Application
}

// GopNew returns a new Application ready to run.
func GopNew(name, wd string, env []string, options func(*source.Options)) *GopApplication {
	app := New(name, wd, env, options)
	return &GopApplication{Application: *app}
}

// DetailedHelp implements tool.Application returning the main binary help.
// This includes the short help for all the sub commands.
func (app *GopApplication) DetailedHelp(f *flag.FlagSet) {
	w := tabwriter.NewWriter(f.Output(), 0, 0, 2, ' ', 0)
	defer w.Flush()

	fmt.Fprint(w, `
goxls is a Go+ language server.

It is typically used with an editor to provide language features. When no
command is specified, goxls will default to the 'serve' command. The language
features can also be accessed via the goxls command-line interface.

Usage:
  goxls help [<subject>]

Command:
`)
	fmt.Fprint(w, "\nMain\t\n")
	for _, c := range app.mainCommands() {
		fmt.Fprintf(w, "  %s\t%s\n", c.Name(), c.ShortHelp())
	}
	fmt.Fprint(w, "\t\nFeatures\t\n")
	for _, c := range app.featureCommands() {
		fmt.Fprintf(w, "  %s\t%s\n", c.Name(), c.ShortHelp())
	}
	fmt.Fprint(w, "\nflags:\n")
	printFlagDefaults(f)
}

// Run takes the args after top level flag processing, and invokes the correct
// sub command as specified by the first argument.
// If no arguments are passed it will invoke the server sub command, as a
// temporary measure for compatibility.
func (app *GopApplication) Run(ctx context.Context, args ...string) error {
	// In the category of "things we can do while waiting for the Go command":
	// Pre-initialize the filecache, which takes ~50ms to hash the goxls
	// executable, and immediately runs a gc.
	filecache.Start()

	ctx = debug.WithInstance(ctx, app.wd, app.OCAgent)
	if len(args) == 0 {
		s := flag.NewFlagSet(app.Name(), flag.ExitOnError)
		return tool.Run(ctx, s, &app.Serve, args)
	}
	command, args := args[0], args[1:]
	for _, c := range app.Commands() {
		if c.Name() == command {
			s := flag.NewFlagSet(app.Name(), flag.ExitOnError)
			return tool.Run(ctx, s, c, args)
		}
	}
	return tool.CommandLineErrorf("Unknown command %v", command)
}

// GopApplication returns the set of commands supported by the goxls tool on the
// command line.
// The command is specified by the first non flag argument.
func (app *GopApplication) Commands() []tool.Application {
	var commands []tool.Application
	commands = append(commands, app.mainCommands()...)
	commands = append(commands, app.featureCommands()...)
	return commands
}

func (app *GopApplication) mainCommands() []tool.Application {
	goApp := &app.Application
	return []tool.Application{
		&app.Serve,
		newGopVersion(app),
		newGopBug(app),
		newGopHelp(app),
		newGopApiJSON(app),
		&licenses{app: goApp},
	}
}

func (app *GopApplication) featureCommands() []tool.Application {
	goApp := &app.Application
	return []tool.Application{
		&callHierarchy{app: goApp},
		&check{app: goApp},
		&definition{app: goApp},
		&foldingRanges{app: goApp},
		&format{app: goApp},
		&highlight{app: goApp},
		&implementation{app: goApp},
		&imports{app: goApp},
		newGopRemote(app, ""),
		newGopRemote(app, "inspect"),
		&links{app: goApp},
		&prepareRename{app: goApp},
		&references{app: goApp},
		&rename{app: goApp},
		&semtok{app: goApp},
		&signature{app: goApp},
		&stats{app: goApp},
		&suggestedFix{app: goApp},
		&symbols{app: goApp},
		&workspaceSymbol{app: goApp},
		&vulncheck{app: goApp},
	}
}
