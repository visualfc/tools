// Copyright 2023 The GoPlus Authors (goplus.org). All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cache

import (
	"fmt"

	"golang.org/x/tools/gopls/internal/goxls/typesutil"
	"golang.org/x/tools/gopls/internal/lsp/protocol"
	"golang.org/x/tools/gopls/internal/lsp/safetoken"
	"golang.org/x/tools/gopls/internal/lsp/source"
	"golang.org/x/tools/gopls/internal/span"
	"golang.org/x/tools/internal/analysisinternal"
	"golang.org/x/tools/internal/typesinternal"
)

func gopTypeErrorDiagnostics(moduleMode bool, linkTarget string, pkg *syntaxPackage, e typesutil.GopTypeError) ([]*source.Diagnostic, error) {
	code, loc, err := gopTypeErrorData(pkg, e)
	if err != nil {
		return nil, err
	}
	diag := &source.Diagnostic{
		URI:      loc.URI.SpanURI(),
		Range:    loc.Range,
		Severity: protocol.SeverityError,
		Source:   source.TypeError,
		Message:  e.Msg,
	}
	if code != 0 {
		diag.Code = code.String()
		diag.CodeHref = typesCodeHref(linkTarget, code)
	}
	switch code {
	case typesinternal.UnusedVar, typesinternal.UnusedImport:
		diag.Tags = append(diag.Tags, protocol.Unnecessary)
	}
	// TODO: diag.SuggestedFixes
	return []*source.Diagnostic{diag}, nil
}

func gopTypeErrorData(pkg *syntaxPackage, terr typesutil.GopTypeError) (typesinternal.ErrorCode, protocol.Location, error) {
	if !terr.Pos.IsValid() {
		return 0, protocol.Location{}, fmt.Errorf("type error (%q) without position", terr.Msg)
	}
	// TODO: It will be better if we can map go+ type error to correct typesinternal.ErrorCode here
	ecode := typesinternal.ErrorCode(0)
	fset := pkg.fset
	start, end := terr.Pos, terr.Pos
	posn := safetoken.StartPosition(fset, start)
	if !posn.IsValid() {
		return 0, protocol.Location{}, fmt.Errorf("position %d of type error %q (code %q) not found in FileSet", start, start, terr)
	}
	pgf, err := pkg.GopFile(span.URIFromPath(posn.Filename))
	if err != nil {
		return 0, protocol.Location{}, err
	}
	if !end.IsValid() || end == start {
		end = analysisinternal.TypeErrorEndPos(fset, pgf.Src, start)
	}
	loc, err := pgf.Mapper.PosLocation(pgf.Tok, start, end)
	return ecode, loc, err
}
