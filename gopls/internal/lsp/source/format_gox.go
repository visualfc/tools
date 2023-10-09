// Copyright 2023 The GoPlus Authors (goplus.org). All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package source

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"

	"github.com/goplus/gop/format"
	"golang.org/x/tools/gopls/internal/goxls/goputil"
	"golang.org/x/tools/gopls/internal/goxls/parserutil"
	"golang.org/x/tools/gopls/internal/lsp/protocol"
	"golang.org/x/tools/internal/event"
	"golang.org/x/tools/internal/tokeninternal"
)

// FormatGop formats a file with a given range.
func FormatGop(ctx context.Context, snapshot Snapshot, fh FileHandle) ([]protocol.TextEdit, error) {
	ctx, done := event.Start(ctx, "gop.Format")
	defer done()

	// Generated files shouldn't be edited. So, don't format them
	if IsGenerated(ctx, snapshot, fh.URI()) {
		return nil, fmt.Errorf("can't format %q: file is generated", fh.URI().Filename())
	}

	pgf, err := snapshot.ParseGop(ctx, fh, parserutil.ParseFull)
	if err != nil {
		return nil, err
	}
	// Even if this file has parse errors, it might still be possible to format it.
	// Using format.Node on an AST with errors may result in code being modified.
	// Attempt to format the source of this file instead.
	if pgf.ParseErr != nil {
		formatted, err := formatGopSource(ctx, fh)
		if err != nil {
			return nil, err
		}
		return computeGopTextEdits(ctx, snapshot, pgf, string(formatted))
	}

	// format.Node changes slightly from one release to another, so the version
	// of Go used to build the LSP server will determine how it formats code.
	// This should be acceptable for all users, who likely be prompted to rebuild
	// the LSP server on each Go release.
	buf := &bytes.Buffer{}
	fset := tokeninternal.FileSetFor(pgf.Tok)
	if err := format.Node(buf, fset, pgf.File); err != nil {
		return nil, err
	}
	formatted := buf.String()

	// Apply additional formatting, if any is supported. Currently, the only
	// supported additional formatter is gofumpt.
	if format := snapshot.Options().GofumptFormat; snapshot.Options().Gofumpt && format != nil {
		// gofumpt can customize formatting based on language version and module
		// path, if available.
		//
		// Try to derive this information, but fall-back on the default behavior.
		//
		// TODO: under which circumstances can we fail to find module information?
		// Can this, for example, result in inconsistent formatting across saves,
		// due to pending calls to packages.Load?
		var langVersion, modulePath string
		meta, err := NarrowestMetadataForFile(ctx, snapshot, fh.URI())
		if err == nil {
			if mi := meta.Module; mi != nil {
				langVersion = mi.GoVersion
				modulePath = mi.Path
			}
		}
		b, err := format(ctx, langVersion, modulePath, buf.Bytes())
		if err != nil {
			return nil, err
		}
		formatted = string(b)
	}
	return computeGopTextEdits(ctx, snapshot, pgf, formatted)
}

func computeGopTextEdits(ctx context.Context, snapshot Snapshot, pgf *ParsedGopFile, formatted string) ([]protocol.TextEdit, error) {
	_, done := event.Start(ctx, "gop.computeTextEdits")
	defer done()

	edits := snapshot.Options().ComputeEdits(string(pgf.Src), formatted)
	return ToProtocolEdits(pgf.Mapper, edits)
}

func formatGopSource(ctx context.Context, fh FileHandle) ([]byte, error) {
	_, done := event.Start(ctx, "gop.formatSource")
	defer done()

	data, err := fh.Content()
	if err != nil {
		return nil, err
	}
	return format.Source(data, isClass(fh))
}

func isClass(fh FileHandle) bool {
	fext := filepath.Ext(fh.URI().Filename())
	return goputil.FileKind(fext) == goputil.FileGopClass
}
