// Copyright 2023 The GoPlus Authors (goplus.org). All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package source

/*
import (
	"context"
	"fmt"
	"go/doc"

	"github.com/goplus/gop/ast"
	"golang.org/x/tools/gopls/internal/bug"
	"golang.org/x/tools/gopls/internal/goxls/parserutil"
	"golang.org/x/tools/gopls/internal/lsp/protocol"
	"golang.org/x/tools/internal/event"
)

// Hover implements the "textDocument/hover" RPC for Go+ files.
func HoverGop(ctx context.Context, snapshot Snapshot, fh FileHandle, position protocol.Position) (*protocol.Hover, error) {
	ctx, done := event.Start(ctx, "gop.Hover")
	defer done()

	rng, h, err := hoverGop(ctx, snapshot, fh, position)
	if err != nil {
		return nil, err
	}
	if h == nil {
		return nil, nil
	}
	hover, err := formatHover(h, snapshot.Options())
	if err != nil {
		return nil, err
	}
	return &protocol.Hover{
		Contents: protocol.MarkupContent{
			Kind:  snapshot.Options().PreferredContentFormat,
			Value: hover,
		},
		Range: rng,
	}, nil
}

// hoverGop computes hover information at the given position. If we do not support
// hovering at the position, it returns _, nil, nil: an error is only returned
// if the position is valid but we fail to compute hover information.
func hoverGop(ctx context.Context, snapshot Snapshot, fh FileHandle, pp protocol.Position) (protocol.Range, *HoverJSON, error) {
	pkg, pgf, err := NarrowestPackageForGopFile(ctx, snapshot, fh.URI())
	if err != nil {
		return protocol.Range{}, nil, err
	}
	pos, err := pgf.PositionPos(pp)
	if err != nil {
		return protocol.Range{}, nil, err
	}

	// Handle hovering over import paths, which do not have an associated
	// identifier.
	for _, spec := range pgf.File.Imports {
		// We are inclusive of the end point here to allow hovering when the cursor
		// is just after the import path.
		if spec.Path.Pos() <= pos && pos <= spec.Path.End() {
			return hoverGopImport(ctx, snapshot, pkg, pgf, spec)
		}
	}

	return protocol.Range{}, nil, nil
}

// hoverGopImport computes hover information when hovering over the import path of
// imp in the file pgf of pkg.
//
// If we do not have metadata for the hovered import, it returns _
func hoverGopImport(ctx context.Context, snapshot Snapshot, pkg Package, pgf *ParsedGopFile, imp *ast.ImportSpec) (protocol.Range, *HoverJSON, error) {
	rng, err := pgf.NodeRange(imp.Path)
	if err != nil {
		return protocol.Range{}, nil, err
	}

	importPath := UnquoteGopImportPath(imp)
	if importPath == "" {
		return protocol.Range{}, nil, fmt.Errorf("invalid import path")
	}
	impID := pkg.Metadata().DepsByImpPath[importPath]
	if impID == "" {
		return protocol.Range{}, nil, fmt.Errorf("no package data for import %q", importPath)
	}
	impMetadata := snapshot.Metadata(impID)
	if impMetadata == nil {
		return protocol.Range{}, nil, bug.Errorf("failed to resolve import ID %q", impID)
	}

	// Find the first file with a package doc comment.
	var comment *ast.CommentGroup
	for _, f := range impMetadata.CompiledGoFiles {
		fh, err := snapshot.ReadFile(ctx, f)
		if err != nil {
			if ctx.Err() != nil {
				return protocol.Range{}, nil, ctx.Err()
			}
			continue
		}
		pgf, err := snapshot.ParseGop(ctx, fh, parserutil.ParseHeader)
		if err != nil {
			if ctx.Err() != nil {
				return protocol.Range{}, nil, ctx.Err()
			}
			continue
		}
		if pgf.File.Doc != nil {
			comment = pgf.File.Doc
			break
		}
	}

	docText := comment.Text()
	return rng, &HoverJSON{
		Synopsis:          doc.Synopsis(docText),
		FullDocumentation: docText,
	}, nil
}
*/
