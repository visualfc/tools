// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package lsp

import (
	"context"

	"golang.org/x/tools/gopls/internal/lsp/protocol"
	"golang.org/x/tools/gopls/internal/lsp/source"
	"golang.org/x/tools/internal/event"
	"golang.org/x/tools/internal/event/tag"
)

func (s *Server) implementation(ctx context.Context, params *protocol.ImplementationParams) ([]protocol.Location, error) {
	ctx, done := event.Start(ctx, "lsp.Server.implementation", tag.URI.Of(params.TextDocument.URI))
	defer done()

	// goxls: Go+
	// snapshot, fh, ok, release, err := s.beginFileRequest(ctx, params.TextDocument.URI, source.Go)
	snapshot, fh, ok, release, err := s.beginFileRequest(ctx, params.TextDocument.URI, source.UnknownKind)
	defer release()
	if !ok {
		return nil, err
	}

	// goxls: Go+
	if kind := snapshot.View().FileKind(fh); kind == source.Gop {
		return source.GopImplementation(ctx, snapshot, fh, params.Position)
	} else if kind != source.Go {
		return nil, nil
	}

	return source.Implementation(ctx, snapshot, fh, params.Position)
}
