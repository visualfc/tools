// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package lsp

import (
	"context"

	"golang.org/x/tools/gopls/internal/lsp/protocol"
	"golang.org/x/tools/gopls/internal/lsp/source"
	"golang.org/x/tools/internal/event"
)

func (s *Server) prepareCallHierarchy(ctx context.Context, params *protocol.CallHierarchyPrepareParams) ([]protocol.CallHierarchyItem, error) {
	ctx, done := event.Start(ctx, "lsp.Server.prepareCallHierarchy")
	defer done()

	// goxls: Go+
	// snapshot, fh, ok, release, err := s.beginFileRequest(ctx, params.TextDocument.URI, source.Go)
	snapshot, fh, ok, release, err := s.beginFileRequest(ctx, params.TextDocument.URI, source.UnknownKind)
	defer release()
	if !ok {
		return nil, err
	}
	// goxls: Go+
	switch kind := snapshot.View().FileKind(fh); kind {
	case source.Gop:
		return source.GopPrepareCallHierarchy(ctx, snapshot, fh, params.Position)
	case source.Go:
		return source.PrepareCallHierarchy(ctx, snapshot, fh, params.Position)
	default:
		return nil, nil
	}
}

func (s *Server) incomingCalls(ctx context.Context, params *protocol.CallHierarchyIncomingCallsParams) ([]protocol.CallHierarchyIncomingCall, error) {
	ctx, done := event.Start(ctx, "lsp.Server.incomingCalls")
	defer done()

	// goxls: Go+
	// snapshot, fh, ok, release, err := s.beginFileRequest(ctx, params.Item.URI, source.Go)
	snapshot, fh, ok, release, err := s.beginFileRequest(ctx, params.Item.URI, source.UnknownKind)
	defer release()
	if !ok {
		return nil, err
	}
	// goxls: Go+
	switch kind := snapshot.View().FileKind(fh); kind {
	case source.Gop:
		return source.GopIncomingCalls(ctx, snapshot, fh, params.Item.Range.Start)
	case source.Go:
		return source.IncomingCalls(ctx, snapshot, fh, params.Item.Range.Start)
	default:
		return nil, nil
	}
}

func (s *Server) outgoingCalls(ctx context.Context, params *protocol.CallHierarchyOutgoingCallsParams) ([]protocol.CallHierarchyOutgoingCall, error) {
	ctx, done := event.Start(ctx, "lsp.Server.outgoingCalls")
	defer done()

	// goxls: Go+
	// snapshot, fh, ok, release, err := s.beginFileRequest(ctx, params.Item.URI, source.Go)
	snapshot, fh, ok, release, err := s.beginFileRequest(ctx, params.Item.URI, source.UnknownKind)
	defer release()
	if !ok {
		return nil, err
	}
	// goxls: Go+
	switch kind := snapshot.View().FileKind(fh); kind {
	case source.Gop:
		return source.GopOutgoingCalls(ctx, snapshot, fh, params.Item.Range.Start)
	case source.Go:
		return source.OutgoingCalls(ctx, snapshot, fh, params.Item.Range.Start)
	default:
		return nil, nil
	}
}
