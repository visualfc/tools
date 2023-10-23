// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package lsp

import (
	"context"
	"log"

	"golang.org/x/tools/gopls/internal/goxls"
	"golang.org/x/tools/gopls/internal/lsp/protocol"
	"golang.org/x/tools/gopls/internal/lsp/source"
	"golang.org/x/tools/gopls/internal/lsp/template"
	"golang.org/x/tools/internal/event"
	"golang.org/x/tools/internal/event/tag"
)

func (s *Server) documentHighlight(ctx context.Context, params *protocol.DocumentHighlightParams) ([]protocol.DocumentHighlight, error) {
	ctx, done := event.Start(ctx, "lsp.Server.documentHighlight", tag.URI.Of(params.TextDocument.URI))
	defer done()

	// goxls: Go+
	// snapshot, fh, ok, release, err := s.beginFileRequest(ctx, params.TextDocument.URI, source.Go)
	snapshot, fh, ok, release, err := s.beginFileRequest(ctx, params.TextDocument.URI, source.UnknownKind)
	defer release()
	if !ok {
		return nil, err
	}

	// goxls: Go+
	// if snapshot.View().FileKind(fh) == source.Tmpl {
	if kind := snapshot.View().FileKind(fh); kind == source.Gop {
		rngs, err := source.GopHighlight(ctx, snapshot, fh, params.Position)
		if goxls.DbgHighlight {
			log.Println("GopHighlight:", fh.URI().Filename(), "err:", err, "len(rngs):", len(rngs))
		}
		if err != nil {
			event.Error(ctx, "no highlight", err)
		}
		return toProtocolHighlight(rngs), nil
	} else if kind == source.Tmpl {
		return template.Highlight(ctx, snapshot, fh, params.Position)
	} else if kind != source.Go { // goxls: Go+
		return nil, nil
	}

	rngs, err := source.Highlight(ctx, snapshot, fh, params.Position)
	if err != nil {
		event.Error(ctx, "no highlight", err)
	}
	if goxls.DbgHighlight {
		log.Println("Highlight:", fh.URI().Filename(), "err:", err, "len(rngs):", len(rngs))
	}
	return toProtocolHighlight(rngs), nil
}

func toProtocolHighlight(rngs []protocol.Range) []protocol.DocumentHighlight {
	result := make([]protocol.DocumentHighlight, 0, len(rngs))
	kind := protocol.Text
	for _, rng := range rngs {
		result = append(result, protocol.DocumentHighlight{
			Kind:  kind,
			Range: rng,
		})
	}
	return result
}
