// Copyright 2023 The GoPlus Authors (goplus.org). All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package source

import (
	"context"

	"golang.org/x/tools/gopls/internal/goxls/parserutil"
	"golang.org/x/tools/gopls/internal/lsp/protocol"
)

// gopParsePackageNameDecl is a convenience function that parses and
// returns the package name declaration of file fh, and reports
// whether the position ppos lies within it.
//
// Note: also used by references.
func gopParsePackageNameDecl(ctx context.Context, snapshot Snapshot, fh FileHandle, ppos protocol.Position) (*ParsedGopFile, bool, error) {
	pgf, err := snapshot.ParseGop(ctx, fh, parserutil.ParseHeader)
	if err != nil {
		return nil, false, err
	}
	// Careful: because we used ParseHeader,
	// pgf.Pos(ppos) may be beyond EOF => (0, err).
	pos, _ := pgf.PositionPos(ppos)
	return pgf, pgf.HasPkgDecl() && pgf.File.Name.Pos() <= pos && pos <= pgf.File.Name.End(), nil
}
