// Copyright 2023 The GoPlus Authors (goplus.org). All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"flag"
	"fmt"
	"text/tabwriter"

	"golang.org/x/tools/gopls/internal/lsp/source"
)

// GopApplication is the main application as passed to tool.Main
// It handles the main command line parsing and dispatch to the sub commands.
type GopApplication struct {
	*Application
}

// GopNew returns a new Application ready to run.
func GopNew(name, wd string, env []string, options func(*source.Options)) GopApplication {
	app := New(name, wd, env, options)
	return GopApplication{app}
}

// DetailedHelp implements tool.Application returning the main binary help.
// This includes the short help for all the sub commands.
func (app GopApplication) DetailedHelp(f *flag.FlagSet) {
	w := tabwriter.NewWriter(f.Output(), 0, 0, 2, ' ', 0)
	defer w.Flush()

	fmt.Fprint(w, `
goxls is a Go+ language server.

It is typically used with an editor to provide language features. When no
command is specified, gopls will default to the 'serve' command. The language
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
