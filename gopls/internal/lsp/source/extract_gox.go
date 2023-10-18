// Copyright 2023 The GoPlus Authors (goplus.org). All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package source

import (
	"fmt"
	"sort"

	"github.com/goplus/gop/ast"
	"github.com/goplus/gop/token"
	"golang.org/x/tools/gopls/internal/goxls/astutil"
	"golang.org/x/tools/gopls/internal/lsp/safetoken"
)

// GopCanExtractVariable reports whether the code in the given range can be
// extracted to a variable.
func GopCanExtractVariable(start, end token.Pos, file *ast.File) (ast.Expr, []ast.Node, bool, error) {
	if start == end {
		return nil, nil, false, fmt.Errorf("start and end are equal")
	}
	path, _ := astutil.PathEnclosingInterval(file, start, end)
	if len(path) == 0 {
		return nil, nil, false, fmt.Errorf("no path enclosing interval")
	}
	for _, n := range path {
		if _, ok := n.(*ast.ImportSpec); ok {
			return nil, nil, false, fmt.Errorf("cannot extract variable in an import block")
		}
	}
	node := path[0]
	if start != node.Pos() || end != node.End() {
		return nil, nil, false, fmt.Errorf("range does not map to an AST node")
	}
	expr, ok := node.(ast.Expr)
	if !ok {
		return nil, nil, false, fmt.Errorf("node is not an expression")
	}
	switch expr.(type) {
	case *ast.BasicLit, *ast.CompositeLit, *ast.IndexExpr, *ast.CallExpr,
		*ast.SliceExpr, *ast.UnaryExpr, *ast.BinaryExpr, *ast.SelectorExpr:
		return expr, path, true, nil
	}
	return nil, nil, false, fmt.Errorf("cannot extract an %T to a variable", expr)
}

type gopFnExtractParams struct {
	tok        *token.File
	start, end token.Pos
	path       []ast.Node
	outer      *ast.FuncDecl
	node       ast.Node
}

// GopCanExtractFunction reports whether the code in the given range can be
// extracted to a function.
func GopCanExtractFunction(tok *token.File, start, end token.Pos, src []byte, file *ast.File) (*gopFnExtractParams, bool, bool, error) {
	if start == end {
		return nil, false, false, fmt.Errorf("start and end are equal")
	}
	var err error
	start, end, err = gopAdjustRangeForCommentsAndWhiteSpace(tok, start, end, src, file)
	if err != nil {
		return nil, false, false, err
	}
	path, _ := astutil.PathEnclosingInterval(file, start, end)
	if len(path) == 0 {
		return nil, false, false, fmt.Errorf("no path enclosing interval")
	}
	// Node that encloses the selection must be a statement.
	// TODO: Support function extraction for an expression.
	_, ok := path[0].(ast.Stmt)
	if !ok {
		return nil, false, false, fmt.Errorf("node is not a statement")
	}

	// Find the function declaration that encloses the selection.
	var outer *ast.FuncDecl
	for _, p := range path {
		if p, ok := p.(*ast.FuncDecl); ok {
			outer = p
			break
		}
	}
	if outer == nil {
		return nil, false, false, fmt.Errorf("no enclosing function")
	}

	// Find the nodes at the start and end of the selection.
	var startNode, endNode ast.Node
	ast.Inspect(outer, func(n ast.Node) bool {
		if n == nil {
			return false
		}
		// Do not override 'start' with a node that begins at the same location
		// but is nested further from 'outer'.
		if startNode == nil && n.Pos() == start && n.End() <= end {
			startNode = n
		}
		if endNode == nil && n.End() == end && n.Pos() >= start {
			endNode = n
		}
		return n.Pos() <= end
	})
	if startNode == nil || endNode == nil {
		return nil, false, false, fmt.Errorf("range does not map to AST nodes")
	}
	// If the region is a blockStmt, use the first and last nodes in the block
	// statement.
	// <rng.start>{ ... }<rng.end> => { <rng.start>...<rng.end> }
	if blockStmt, ok := startNode.(*ast.BlockStmt); ok {
		if len(blockStmt.List) == 0 {
			return nil, false, false, fmt.Errorf("range maps to empty block statement")
		}
		startNode, endNode = blockStmt.List[0], blockStmt.List[len(blockStmt.List)-1]
		start, end = startNode.Pos(), endNode.End()
	}
	return &gopFnExtractParams{
		tok:   tok,
		start: start,
		end:   end,
		path:  path,
		outer: outer,
		node:  startNode,
	}, true, outer.Recv != nil, nil
}

// gopAdjustRangeForCommentsAndWhiteSpace adjusts the given range to exclude unnecessary leading or
// trailing whitespace characters from selection as well as leading or trailing comments.
// In the following example, each line of the if statement is indented once. There are also two
// extra spaces after the sclosing bracket before the line break and a comment.
//
// \tif (true) {
// \t    _ = 1
// \t} // hello \n
//
// By default, a valid range begins at 'if' and ends at the first whitespace character
// after the '}'. But, users are likely to highlight full lines rather than adjusting
// their cursors for whitespace. To support this use case, we must manually adjust the
// ranges to match the correct AST node. In this particular example, we would adjust
// rng.Start forward to the start of 'if' and rng.End backward to after '}'.
func gopAdjustRangeForCommentsAndWhiteSpace(tok *token.File, start, end token.Pos, content []byte, file *ast.File) (token.Pos, token.Pos, error) {
	// Adjust the end of the range to after leading whitespace and comments.
	prevStart := token.NoPos
	startComment := sort.Search(len(file.Comments), func(i int) bool {
		// Find the index for the first comment that ends after range start.
		return file.Comments[i].End() > start
	})
	for prevStart != start {
		prevStart = start
		// If start is within a comment, move start to the end
		// of the comment group.
		if startComment < len(file.Comments) && file.Comments[startComment].Pos() <= start && start < file.Comments[startComment].End() {
			start = file.Comments[startComment].End()
			startComment++
		}
		// Move forwards to find a non-whitespace character.
		offset, err := safetoken.Offset(tok, start)
		if err != nil {
			return 0, 0, err
		}
		for offset < len(content) && isGoWhiteSpace(content[offset]) {
			offset++
		}
		start = tok.Pos(offset)
	}

	// Adjust the end of the range to before trailing whitespace and comments.
	prevEnd := token.NoPos
	endComment := sort.Search(len(file.Comments), func(i int) bool {
		// Find the index for the first comment that ends after the range end.
		return file.Comments[i].End() >= end
	})
	// Search will return n if not found, so we need to adjust if there are no
	// comments that would match.
	if endComment == len(file.Comments) {
		endComment = -1
	}
	for prevEnd != end {
		prevEnd = end
		// If end is within a comment, move end to the start
		// of the comment group.
		if endComment >= 0 && file.Comments[endComment].Pos() < end && end <= file.Comments[endComment].End() {
			end = file.Comments[endComment].Pos()
			endComment--
		}
		// Move backwards to find a non-whitespace character.
		offset, err := safetoken.Offset(tok, end)
		if err != nil {
			return 0, 0, err
		}
		for offset > 0 && isGoWhiteSpace(content[offset-1]) {
			offset--
		}
		end = tok.Pos(offset)
	}

	return start, end, nil
}
