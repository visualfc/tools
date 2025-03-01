// Copyright 2023 The GoPlus Authors (goplus.org). All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package completion

import (
	"context"
	"fmt"
	"go/constant"
	"go/types"
	"log"
	"math"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/goplus/gop/ast"
	"github.com/goplus/gop/printer"
	"github.com/goplus/gop/scanner"
	"github.com/goplus/gop/token"
	"github.com/goplus/gop/x/typesutil"
	"golang.org/x/sync/errgroup"
	"golang.org/x/tools/gop/ast/astutil"
	"golang.org/x/tools/gopls/internal/goxls"
	goxlsastutil "golang.org/x/tools/gopls/internal/goxls/astutil"
	"golang.org/x/tools/gopls/internal/goxls/parserutil"
	"golang.org/x/tools/gopls/internal/lsp/protocol"
	"golang.org/x/tools/gopls/internal/lsp/safetoken"
	"golang.org/x/tools/gopls/internal/lsp/snippet"
	"golang.org/x/tools/gopls/internal/lsp/source"
	"golang.org/x/tools/gopls/internal/span"
	"golang.org/x/tools/internal/event"
	"golang.org/x/tools/internal/fuzzy"
	"golang.org/x/tools/internal/gop/typeparams"
	"golang.org/x/tools/internal/imports"
)

// gopCompleter contains the necessary information for a single completion request.
type gopCompleter struct {
	snapshot source.Snapshot
	pkg      source.Package
	qf       types.Qualifier          // for qualifying typed expressions
	mq       source.MetadataQualifier // for syntactic qualifying
	opts     *completionOptions

	// completionContext contains information about the trigger for this
	// completion request.
	completionContext completionContext

	// fh is a handle to the file associated with this completion request.
	fh source.FileHandle

	// filename is the name of the file associated with this completion request.
	filename string

	// file is the AST of the file associated with this completion request.
	file *ast.File

	// (tokFile, pos) is the position at which the request was triggered.
	tokFile *token.File
	pos     token.Pos

	// path is the path of AST nodes enclosing the position.
	path []ast.Node

	// seen is the map that ensures we do not return duplicate results.
	seen map[types.Object]bool

	// items is the list of completion items returned.
	items []CompletionItem

	// completionCallbacks is a list of callbacks to collect completions that
	// require expensive operations. This includes operations where we search
	// through the entire module cache.
	completionCallbacks []func(context.Context, *imports.Options) error

	// surrounding describes the identifier surrounding the position.
	surrounding *Selection

	// inference contains information we've inferred about ideal
	// candidates such as the candidate's type.
	inference candidateInference

	// enclosingFunc contains information about the function enclosing
	// the position.
	enclosingFunc *gopFuncInfo

	// enclosingCompositeLiteral contains information about the composite literal
	// enclosing the position.
	enclosingCompositeLiteral *gopCompLitInfo

	// deepState contains the current state of our deep completion search.
	deepState deepCompletionState

	// matcher matches the candidates against the surrounding prefix.
	matcher matcher

	// methodSetCache caches the types.NewMethodSet call, which is relatively
	// expensive and can be called many times for the same type while searching
	// for deep completions.
	methodSetCache map[methodSetKey]*types.MethodSet

	// mapper converts the positions in the file from which the completion originated.
	mapper *protocol.Mapper

	// startTime is when we started processing this completion request. It does
	// not include any time the request spent in the queue.
	//
	// Note: in CL 503016, startTime move to *after* type checking, but it was
	// subsequently determined that it was better to keep setting it *before*
	// type checking, so that the completion budget best approximates the user
	// experience. See golang/go#62665 for more details.
	// startTime time.Time

	// scopes contains all scopes defined by nodes in our path,
	// including nil values for nodes that don't defined a scope. It
	// also includes our package scope and the universal scope at the
	// end.
	scopes []*types.Scope
}

// gopFuncInfo holds info about a function object.
type gopFuncInfo struct {
	// sig is the function declaration enclosing the position.
	sig *types.Signature

	// body is the function's body.
	body *ast.BlockStmt
}

type gopCompLitInfo struct {
	// cl is the *ast.CompositeLit enclosing the position.
	cl *ast.CompositeLit

	// clType is the type of cl.
	clType types.Type

	// kv is the *ast.KeyValueExpr enclosing the position, if any.
	kv *ast.KeyValueExpr

	// inKey is true if we are certain the position is in the key side
	// of a key-value pair.
	inKey bool

	// maybeInFieldName is true if inKey is false and it is possible
	// we are completing a struct field name. For example,
	// "SomeStruct{<>}" will be inKey=false, but maybeInFieldName=true
	// because we _could_ be completing a field name.
	maybeInFieldName bool
}

// gopEnclosingCompositeLiteral returns information about the composite literal enclosing the
// position.
func gopEnclosingCompositeLiteral(path []ast.Node, pos token.Pos, info *typesutil.Info) *gopCompLitInfo {
	for _, n := range path {
		switch n := n.(type) {
		case *ast.CompositeLit:
			// The enclosing node will be a composite literal if the user has just
			// opened the curly brace (e.g. &x{<>) or the completion request is triggered
			// from an already completed composite literal expression (e.g. &x{foo: 1, <>})
			//
			// The position is not part of the composite literal unless it falls within the
			// curly braces (e.g. "foo.Foo<>Struct{}").
			if !(n.Lbrace < pos && pos <= n.Rbrace) {
				// Keep searching since we may yet be inside a composite literal.
				// For example "Foo{B: Ba<>{}}".
				break
			}

			tv, ok := info.Types[n]
			if !ok {
				return nil
			}

			clInfo := gopCompLitInfo{
				cl:     n,
				clType: source.Deref(tv.Type).Underlying(),
			}

			var (
				expr    ast.Expr
				hasKeys bool
			)
			for _, el := range n.Elts {
				// Remember the expression that the position falls in, if any.
				if el.Pos() <= pos && pos <= el.End() {
					expr = el
				}

				if kv, ok := el.(*ast.KeyValueExpr); ok {
					hasKeys = true
					// If expr == el then we know the position falls in this expression,
					// so also record kv as the enclosing *ast.KeyValueExpr.
					if expr == el {
						clInfo.kv = kv
						break
					}
				}
			}

			if clInfo.kv != nil {
				// If in a *ast.KeyValueExpr, we know we are in the key if the position
				// is to the left of the colon (e.g. "Foo{F<>: V}".
				clInfo.inKey = pos <= clInfo.kv.Colon
			} else if hasKeys {
				// If we aren't in a *ast.KeyValueExpr but the composite literal has
				// other *ast.KeyValueExprs, we must be on the key side of a new
				// *ast.KeyValueExpr (e.g. "Foo{F: V, <>}").
				clInfo.inKey = true
			} else {
				switch clInfo.clType.(type) {
				case *types.Struct:
					if len(n.Elts) == 0 {
						// If the struct literal is empty, next could be a struct field
						// name or an expression (e.g. "Foo{<>}" could become "Foo{F:}"
						// or "Foo{someVar}").
						clInfo.maybeInFieldName = true
					} else if len(n.Elts) == 1 {
						// If there is one expression and the position is in that expression
						// and the expression is an identifier, we may be writing a field
						// name or an expression (e.g. "Foo{F<>}").
						_, clInfo.maybeInFieldName = expr.(*ast.Ident)
					}
				case *types.Map:
					// If we aren't in a *ast.KeyValueExpr we must be adding a new key
					// to the map.
					clInfo.inKey = true
				}
			}

			return &clInfo
		default:
			if breaksExpectedTypeInference(n, pos) {
				return nil
			}
		}
	}

	return nil
}

// gopEnclosingFunction returns the signature and body of the function
// enclosing the given position.
func gopEnclosingFunction(path []ast.Node, info *typesutil.Info) *gopFuncInfo {
	for _, node := range path {
		switch t := node.(type) {
		case *ast.FuncDecl:
			if obj, ok := info.Defs[t.Name]; ok {
				return &gopFuncInfo{
					sig:  obj.Type().(*types.Signature),
					body: t.Body,
				}
			}
		case *ast.FuncLit:
			if typ, ok := info.Types[t]; ok {
				if sig, _ := typ.Type.(*types.Signature); sig == nil {
					// golang/go#49397: it should not be possible, but we somehow arrived
					// here with a non-signature type, most likely due to AST mangling
					// such that node.Type is not a FuncType.
					return nil
				}
				return &gopFuncInfo{
					sig:  typ.Type.(*types.Signature),
					body: t.Body,
				}
			}
		}
	}
	return nil
}

// GopCompletion returns a list of possible candidates for completion, given a
// a file and a position.
//
// The selection is computed based on the preceding identifier and can be used by
// the client to score the quality of the completion. For instance, some clients
// may tolerate imperfect matches as valid completion results, since users may make typos.
func GopCompletion(ctx context.Context, snapshot source.Snapshot, fh source.FileHandle, protoPos protocol.Position, protoContext protocol.CompletionContext) ([]CompletionItem, *Selection, error) {
	if goxls.DbgCompletion {
		log.Println("GopCompletion:", fh.URI().Filename(), "kind:", protoContext.TriggerKind, "triggerCh:", protoContext.TriggerCharacter)
		defer log.Println("GopCompletion done:", fh.URI().Filename(), "kind:", protoContext.TriggerKind, "triggerCh:", protoContext.TriggerCharacter)
	}
	ctx, done := event.Start(ctx, "completion.GopCompletion")
	defer done()

	pkg, pgf, err := source.NarrowestPackageForGopFile(ctx, snapshot, fh.URI())
	if err != nil || (pgf.File.Package == token.NoPos && pkg.GetTypes().Name() != "main") {
		// If we can't parse this file or find position for the package
		// keyword, it may be missing a package declaration. Try offering
		// suggestions for the package declaration.
		// Note that this would be the case even if the keyword 'package' is
		// present but no package name exists.
		items, surrounding, innerErr := gopPackageClauseCompletions(ctx, snapshot, fh, protoPos)
		if innerErr != nil {
			// return the error for GetParsedFile since it's more relevant in this situation.
			return nil, nil, fmt.Errorf("getting file %s for Completion: %w (package completions: %v)", fh.URI(), err, innerErr)
		}
		return items, surrounding, nil
	}
	pos, err := pgf.PositionPos(protoPos)
	if err != nil {
		return nil, nil, err
	}
	// Completion is based on what precedes the cursor.
	// Find the path to the position before pos.
	path, _ := astutil.PathEnclosingInterval(pgf.File, pos-1, pos-1)
	if path == nil {
		return nil, nil, fmt.Errorf("cannot find node enclosing position")
	}
	if goxls.DbgCompletion {
		log.Printf("GopCompletion PathEnclosingInterval: %T\n", path[0])
	}

	// Check if completion at this position is valid. If not, return early.
	switch n := path[0].(type) {
	case *ast.BasicLit:
		// Skip completion inside literals except for ImportSpec
		if len(path) > 1 {
			if _, ok := path[1].(*ast.ImportSpec); ok {
				break
			}
		}
		return nil, nil, nil
	case *ast.CallExpr:
		if n.Ellipsis.IsValid() && pos > n.Ellipsis && pos <= n.Ellipsis+token.Pos(len("...")) {
			// Don't offer completions inside or directly after "...". For
			// example, don't offer completions at "<>" in "foo(bar...<>").
			return nil, nil, nil
		}
	case *ast.Ident:
		// reject defining identifiers
		obj, ok := pkg.GopTypesInfo().Defs[n]
		if goxls.DbgCompletion {
			log.Println("GopCompletion ident:", n, "obj:", obj)
		}
		if ok {
			if v, ok := obj.(*types.Var); ok && v.IsField() && v.Embedded() {
				// An anonymous field is also a reference to a type.
			} else if pgf.File.Name == n {
				// Don't skip completions if Ident is for package name.
				break
			} else {
				objStr := ""
				if obj != nil {
					qual := types.RelativeTo(pkg.GetTypes())
					objStr = types.ObjectString(obj, qual)
				}
				ans, sel := gopDefinition(path, obj, pgf)
				if ans != nil {
					sort.Slice(ans, func(i, j int) bool {
						return ans[i].Score > ans[j].Score
					})
					return ans, sel, nil
				}
				return nil, nil, ErrIsDefinition{objStr: objStr}
			}
		}
	}

	// Collect all surrounding scopes, innermost first.
	scopes := source.GopCollectScopes(pkg.GopTypesInfo(), path, pos)
	scopes = append(scopes, pkg.GetTypes().Scope(), types.Universe)

	opts := snapshot.View().Options()
	c := &gopCompleter{
		pkg:      pkg,
		snapshot: snapshot,
		qf:       source.GopQualifier(pgf.File, pkg.GetTypes(), pkg.GopTypesInfo()),
		mq:       source.MetadataQualifierForGopFile(snapshot, pgf.File, pkg.Metadata()),
		completionContext: completionContext{
			triggerCharacter: protoContext.TriggerCharacter,
			triggerKind:      protoContext.TriggerKind,
		},
		fh:                        fh,
		filename:                  fh.URI().Filename(),
		tokFile:                   pgf.Tok,
		file:                      pgf.File,
		path:                      path,
		pos:                       pos,
		seen:                      make(map[types.Object]bool),
		enclosingFunc:             gopEnclosingFunction(path, pkg.GopTypesInfo()),
		enclosingCompositeLiteral: gopEnclosingCompositeLiteral(path, pos, pkg.GopTypesInfo()),
		deepState: deepCompletionState{
			enabled: opts.DeepCompletion,
		},
		opts: &completionOptions{
			matcher:           opts.Matcher,
			unimported:        opts.CompleteUnimported,
			documentation:     opts.CompletionDocumentation && opts.HoverKind != source.NoDocumentation,
			fullDocumentation: opts.HoverKind == source.FullDocumentation,
			placeholders:      opts.UsePlaceholders,
			literal:           opts.LiteralCompletions && opts.InsertTextFormat == protocol.SnippetTextFormat,
			budget:            opts.CompletionBudget,
			snippets:          opts.InsertTextFormat == protocol.SnippetTextFormat,
			postfix:           opts.ExperimentalPostfixCompletions,
		},
		// default to a matcher that always matches
		matcher:        prefixMatcher(""),
		methodSetCache: make(map[methodSetKey]*types.MethodSet),
		mapper:         pgf.Mapper,
		scopes:         scopes,
	}

	ctx, cancel := context.WithCancel(ctx)

	// Compute the deadline for this operation. Deadline is relative to the
	// search operation, not the entire completion RPC, as the work up until this
	// point depends significantly on how long it took to type-check, which in
	// turn depends on the timing of the request relative to other operations on
	// the snapshot. Including that work in the budget leads to inconsistent
	// results (and realistically, if type-checking took 200ms already, the user
	// is unlikely to be significantly more bothered by e.g. another 100ms of
	// search).
	//
	// Don't overload the context with this deadline, as we don't want to
	// conflate user cancellation (=fail the operation) with our time limit
	// (=stop searching and succeed with partial results).
	start := time.Now()
	var deadline *time.Time
	if c.opts.budget > 0 {
		d := start.Add(c.opts.budget)
		deadline = &d
	}

	defer cancel()

	if surrounding := c.containingIdent(pgf.Src); surrounding != nil {
		if goxls.DbgCompletion {
			log.Println("GopCompletion surrounding:", surrounding)
		}
		c.setSurrounding(surrounding)
	}

	c.inference = gopExpectedCandidate(ctx, c)
	if goxls.DbgCompletion {
		infer := c.inference
		log.Printf(`GopCompletion infer:
	  objType: %v, convertibleTo: %v
	  objKind: %d, variadic: %v
	  assignees: %v, variadicAssignees: %v
	  objChain: %v
	`, infer.objType, infer.convertibleTo, infer.objKind, infer.variadic,
			infer.assignees, infer.variadicAssignees, infer.objChain)
	}

	err = c.collectCompletions(ctx)
	if err != nil {
		return nil, nil, err
	}
	if goxls.DbgCompletion {
		log.Println("GopCompletion collect: len(items) =", len(c.items))
	}

	// Deep search collected candidates and their members for more candidates.
	c.deepSearch(ctx, start, deadline)
	if goxls.DbgCompletion {
		log.Println("GopCompletion deepSearch: len(items) =", len(c.items))
	}

	for _, callback := range c.completionCallbacks {
		if err := c.snapshot.RunProcessEnvFunc(ctx, callback); err != nil {
			return nil, nil, err
		}
		if goxls.DbgCompletion {
			log.Println("GopCompletion callbak: len(items) =", len(c.items))
		}
	}

	// Search candidates populated by expensive operations like
	// unimportedMembers etc. for more completion items.
	c.deepSearch(ctx, start, deadline)
	if goxls.DbgCompletion {
		log.Println("GopCompletion deepSearch(2): len(items) =", len(c.items))
	}

	// Statement candidates offer an entire statement in certain contexts, as
	// opposed to a single object. Add statement candidates last because they
	// depend on other candidates having already been collected.
	c.addStatementCandidates()

	c.sortItems()
	if goxls.DbgCompletion {
		log.Println("GopCompletion ret: len(items) =", len(c.items))
	}
	return c.items, c.getSurrounding(), nil
}

func (c *gopCompleter) getSurrounding() *Selection {
	if c.surrounding == nil {
		c.surrounding = &Selection{
			content: "",
			cursor:  c.pos,
			tokFile: c.tokFile,
			start:   c.pos,
			end:     c.pos,
			mapper:  c.mapper,
		}
	}
	return c.surrounding
}

// collectCompletions adds possible completion candidates to either the deep
// search queue or completion items directly for different completion contexts.
func (c *gopCompleter) collectCompletions(ctx context.Context) error {
	// Inside import blocks, return completions for unimported packages.
	for _, importSpec := range c.file.Imports {
		if !(importSpec.Path.Pos() <= c.pos && c.pos <= importSpec.Path.End()) {
			continue
		}
		return c.populateImportCompletions(importSpec)
	}

	// Inside comments, offer completions for the name of the relevant symbol.
	for _, comment := range c.file.Comments {
		if comment.Pos() < c.pos && c.pos <= comment.End() {
			c.populateCommentCompletions(ctx, comment)
			return nil
		}
	}

	// Struct literals are handled entirely separately.
	if c.wantStructFieldCompletions() {
		// If we are definitely completing a struct field name, deep completions
		// don't make sense.
		if c.enclosingCompositeLiteral.inKey {
			c.deepState.enabled = false
		}
		return c.structLiteralFieldName(ctx)
	}

	if lt := c.wantLabelCompletion(); lt != labelNone {
		c.labels(lt)
		return nil
	}

	if c.emptySwitchStmt() {
		// Empty switch statements only admit "default" and "case" keywords.
		c.addKeywordItems(map[string]bool{}, highScore, CASE, DEFAULT)
		return nil
	}

	if goxls.DbgCompletion {
		log.Println("collectCompletions: path", reflect.TypeOf(c.path[0]))
	}
	switch n := c.path[0].(type) {
	case *ast.Ident:
		if c.file.Name == n {
			return c.packageNameCompletions(ctx, c.fh.URI(), n)
		} else if sel, ok := c.path[1].(*ast.SelectorExpr); ok && sel.Sel == n {
			// Is this the Sel part of a selector?
			return c.selector(ctx, sel)
		}
		return c.lexical(ctx)
	// The function name hasn't been typed yet, but the parens are there:
	//   recv.‸(arg)
	case *ast.TypeAssertExpr:
		// Create a fake selector expression.
		//
		// The name "_" is the convention used by go/parser to represent phantom
		// selectors.
		sel := &ast.Ident{NamePos: n.X.End() + token.Pos(len(".")), Name: "_"}
		return c.selector(ctx, &ast.SelectorExpr{X: n.X, Sel: sel})
	case *ast.SelectorExpr:
		return c.selector(ctx, n)
	// At the file scope, only keywords are allowed.
	case *ast.BadDecl, *ast.File:
		c.addKeywordCompletions()
	default:
		// fallback to lexical completions
		return c.lexical(ctx)
	}

	return nil
}

// containingIdent returns the *ast.Ident containing pos, if any. It
// synthesizes an *ast.Ident to allow completion in the face of
// certain syntax errors.
func (c *gopCompleter) containingIdent(src []byte) *ast.Ident {
	// In the normal case, our leaf AST node is the identifier being completed.
	if ident, ok := c.path[0].(*ast.Ident); ok {
		return ident
	}

	pos, tkn, lit := c.scanToken(src)
	if !pos.IsValid() {
		return nil
	}

	fakeIdent := &ast.Ident{Name: lit, NamePos: pos}

	if _, isBadDecl := c.path[0].(*ast.BadDecl); isBadDecl {
		// You don't get *ast.Idents at the file level, so look for bad
		// decls and use the manually extracted token.
		return fakeIdent
	} else if c.emptySwitchStmt() {
		// Only keywords are allowed in empty switch statements.
		// *ast.Idents are not parsed, so we must use the manually
		// extracted token.
		return fakeIdent
	} else if tkn.IsKeyword() {
		// Otherwise, manually extract the prefix if our containing token
		// is a keyword. This improves completion after an "accidental
		// keyword", e.g. completing to "variance" in "someFunc(var<>)".
		return fakeIdent
	}

	return nil
}

// scanToken scans pgh's contents for the token containing pos.
func (c *gopCompleter) scanToken(contents []byte) (token.Pos, token.Token, string) {
	tok := c.pkg.FileSet().File(c.pos)

	var s scanner.Scanner
	s.Init(tok, contents, nil, 0)
	for {
		tknPos, tkn, lit := s.Scan()
		if tkn == token.EOF || tknPos >= c.pos {
			return token.NoPos, token.ILLEGAL, ""
		}

		if len(lit) > 0 && tknPos <= c.pos && c.pos <= tknPos+token.Pos(len(lit)) {
			return tknPos, tkn, lit
		}
	}
}

func (c *gopCompleter) sortItems() {
	sort.SliceStable(c.items, func(i, j int) bool {
		// Sort by score first.
		if c.items[i].Score != c.items[j].Score {
			return c.items[i].Score > c.items[j].Score
		}

		// Then sort by label so order stays consistent. This also has the
		// effect of preferring shorter candidates.
		return c.items[i].Label < c.items[j].Label
	})
}

func (c *gopCompleter) fakeObj(T types.Type) *types.Var {
	return types.NewVar(token.NoPos, c.pkg.GetTypes(), "", T)
}

// emptySwitchStmt reports whether pos is in an empty switch or select
// statement.
func (c *gopCompleter) emptySwitchStmt() bool {
	block, ok := c.path[0].(*ast.BlockStmt)
	if !ok || len(block.List) > 0 || len(c.path) == 1 {
		return false
	}

	switch c.path[1].(type) {
	case *ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.SelectStmt:
		return true
	default:
		return false
	}
}

// populateImportCompletions yields completions for an import path around the cursor.
//
// Completions are suggested at the directory depth of the given import path so
// that we don't overwhelm the user with a large list of possibilities. As an
// example, a completion for the prefix "golang" results in "golang.org/".
// Completions for "golang.org/" yield its subdirectories
// (i.e. "golang.org/x/"). The user is meant to accept completion suggestions
// until they reach a complete import path.
func (c *gopCompleter) populateImportCompletions(searchImport *ast.ImportSpec) error {
	if !strings.HasPrefix(searchImport.Path.Value, `"`) {
		return nil
	}

	// deepSearch is not valuable for import completions.
	c.deepState.enabled = false

	importPath := searchImport.Path.Value

	// Extract the text between the quotes (if any) in an import spec.
	// prefix is the part of import path before the cursor.
	prefixEnd := c.pos - searchImport.Path.Pos()
	prefix := strings.Trim(importPath[:prefixEnd], `"`)

	// The number of directories in the import path gives us the depth at
	// which to search.
	depth := len(strings.Split(prefix, "/")) - 1

	content := importPath
	start, end := searchImport.Path.Pos(), searchImport.Path.End()
	namePrefix, nameSuffix := `"`, `"`
	// If a starting quote is present, adjust surrounding to either after the
	// cursor or after the first slash (/), except if cursor is at the starting
	// quote. Otherwise we provide a completion including the starting quote.
	if strings.HasPrefix(importPath, `"`) && c.pos > searchImport.Path.Pos() {
		content = content[1:]
		start++
		if depth > 0 {
			// Adjust textEdit start to replacement range. For ex: if current
			// path was "golang.or/x/to<>ols/internal/", where <> is the cursor
			// position, start of the replacement range would be after
			// "golang.org/x/".
			path := strings.SplitAfter(prefix, "/")
			numChars := len(strings.Join(path[:len(path)-1], ""))
			content = content[numChars:]
			start += token.Pos(numChars)
		}
		namePrefix = ""
	}

	// We won't provide an ending quote if one is already present, except if
	// cursor is after the ending quote but still in import spec. This is
	// because cursor has to be in our textEdit range.
	if strings.HasSuffix(importPath, `"`) && c.pos < searchImport.Path.End() {
		end--
		content = content[:len(content)-1]
		nameSuffix = ""
	}

	c.surrounding = &Selection{
		content: content,
		cursor:  c.pos,
		tokFile: c.tokFile,
		start:   start,
		end:     end,
		mapper:  c.mapper,
	}

	seenImports := make(map[string]struct{})
	for _, importSpec := range c.file.Imports {
		if importSpec.Path.Value == importPath {
			continue
		}
		seenImportPath, err := strconv.Unquote(importSpec.Path.Value)
		if err != nil {
			return err
		}
		seenImports[seenImportPath] = struct{}{}
	}

	var mu sync.Mutex // guard c.items locally, since searchImports is called in parallel
	seen := make(map[string]struct{})
	searchImports := func(pkg imports.ImportFix) {
		path := pkg.StmtInfo.ImportPath
		if _, ok := seenImports[path]; ok {
			return
		}

		// Any package path containing fewer directories than the search
		// prefix is not a match.
		pkgDirList := strings.Split(path, "/")
		if len(pkgDirList) < depth+1 {
			return
		}
		pkgToConsider := strings.Join(pkgDirList[:depth+1], "/")

		name := pkgDirList[depth]
		// if we're adding an opening quote to completion too, set name to full
		// package path since we'll need to overwrite that range.
		if namePrefix == `"` {
			name = pkgToConsider
		}

		score := pkg.Relevance
		if len(pkgDirList)-1 == depth {
			score *= highScore
		} else {
			// For incomplete package paths, add a terminal slash to indicate that the
			// user should keep triggering completions.
			name += "/"
			pkgToConsider += "/"
		}

		if _, ok := seen[pkgToConsider]; ok {
			return
		}
		seen[pkgToConsider] = struct{}{}

		mu.Lock()
		defer mu.Unlock()

		name = namePrefix + name + nameSuffix
		obj := types.NewPkgName(0, nil, name, types.NewPackage(pkgToConsider, name))
		c.deepState.enqueue(candidate{
			obj:    obj,
			detail: fmt.Sprintf("%q", pkgToConsider),
			score:  score,
		})
	}

	c.completionCallbacks = append(c.completionCallbacks, func(ctx context.Context, opts *imports.Options) error {
		return imports.GetImportPaths(ctx, searchImports, prefix, c.filename, c.pkg.GetTypes().Name(), opts.Env)
	})
	return nil
}

// populateCommentCompletions yields completions for comments preceding or in declarations.
func (c *gopCompleter) populateCommentCompletions(ctx context.Context, comment *ast.CommentGroup) {
	// If the completion was triggered by a period, ignore it. These types of
	// completions will not be useful in comments.
	if c.completionContext.triggerCharacter == "." {
		return
	}

	// Using the comment position find the line after
	file := c.pkg.FileSet().File(comment.End())
	if file == nil {
		return
	}

	// Deep completion doesn't work properly in comments since we don't
	// have a type object to complete further.
	c.deepState.enabled = false
	c.completionContext.commentCompletion = true

	// Documentation isn't useful in comments, since it might end up being the
	// comment itself.
	c.opts.documentation = false

	commentLine := safetoken.Line(file, comment.End())

	// comment is valid, set surrounding as word boundaries around cursor
	c.setSurroundingForComment(comment)

	// Using the next line pos, grab and parse the exported symbol on that line
	for _, n := range c.file.Decls {
		declLine := safetoken.Line(file, n.Pos())
		// if the comment is not in, directly above or on the same line as a declaration
		if declLine != commentLine && declLine != commentLine+1 &&
			!(n.Pos() <= comment.Pos() && comment.End() <= n.End()) {
			continue
		}
		switch node := n.(type) {
		// handle const, vars, and types
		case *ast.GenDecl:
			for _, spec := range node.Specs {
				switch spec := spec.(type) {
				case *ast.ValueSpec:
					for _, name := range spec.Names {
						if name.String() == "_" {
							continue
						}
						obj := c.pkg.GopTypesInfo().ObjectOf(name)
						c.deepState.enqueue(candidate{obj: obj, score: stdScore})
					}
				case *ast.TypeSpec:
					// add TypeSpec fields to completion
					switch typeNode := spec.Type.(type) {
					case *ast.StructType:
						c.addFieldItems(ctx, typeNode.Fields)
					case *ast.FuncType:
						c.addFieldItems(ctx, typeNode.Params)
						c.addFieldItems(ctx, typeNode.Results)
					case *ast.InterfaceType:
						c.addFieldItems(ctx, typeNode.Methods)
					}

					if spec.Name.String() == "_" {
						continue
					}

					obj := c.pkg.GopTypesInfo().ObjectOf(spec.Name)
					// Type name should get a higher score than fields but not highScore by default
					// since field near a comment cursor gets a highScore
					score := stdScore * 1.1
					// If type declaration is on the line after comment, give it a highScore.
					if declLine == commentLine+1 {
						score = highScore
					}

					c.deepState.enqueue(candidate{obj: obj, score: score})
				}
			}
		// handle functions
		case *ast.FuncDecl:
			c.addFieldItems(ctx, node.Recv)
			c.addFieldItems(ctx, node.Type.Params)
			c.addFieldItems(ctx, node.Type.Results)

			// collect receiver struct fields
			if node.Recv != nil {
				for _, fields := range node.Recv.List {
					for _, name := range fields.Names {
						obj := c.pkg.GopTypesInfo().ObjectOf(name)
						if obj == nil {
							continue
						}

						recvType := obj.Type().Underlying()
						if ptr, ok := recvType.(*types.Pointer); ok {
							recvType = ptr.Elem()
						}
						recvStruct, ok := recvType.Underlying().(*types.Struct)
						if !ok {
							continue
						}
						for i := 0; i < recvStruct.NumFields(); i++ {
							field := recvStruct.Field(i)
							c.deepState.enqueue(candidate{obj: field, score: lowScore})
						}
					}
				}
			}

			if node.Name.String() == "_" {
				continue
			}

			obj := c.pkg.GopTypesInfo().ObjectOf(node.Name)
			if obj == nil || obj.Pkg() != nil && obj.Pkg() != c.pkg.GetTypes() {
				continue
			}

			c.deepState.enqueue(candidate{obj: obj, score: highScore})
		}
	}
}

// sets word boundaries surrounding a cursor for a comment
func (c *gopCompleter) setSurroundingForComment(comments *ast.CommentGroup) {
	var cursorComment *ast.Comment
	for _, comment := range comments.List {
		if c.pos >= comment.Pos() && c.pos <= comment.End() {
			cursorComment = comment
			break
		}
	}
	// if cursor isn't in the comment
	if cursorComment == nil {
		return
	}

	// index of cursor in comment text
	cursorOffset := int(c.pos - cursorComment.Pos())
	start, end := cursorOffset, cursorOffset
	for start > 0 && isValidIdentifierChar(cursorComment.Text[start-1]) {
		start--
	}
	for end < len(cursorComment.Text) && isValidIdentifierChar(cursorComment.Text[end]) {
		end++
	}

	c.surrounding = &Selection{
		content: cursorComment.Text[start:end],
		cursor:  c.pos,
		tokFile: c.tokFile,
		start:   token.Pos(int(cursorComment.Slash) + start),
		end:     token.Pos(int(cursorComment.Slash) + end),
		mapper:  c.mapper,
	}
	c.setMatcherFromPrefix(c.surrounding.Prefix())
}

// adds struct fields, interface methods, function declaration fields to completion
func (c *gopCompleter) addFieldItems(ctx context.Context, fields *ast.FieldList) {
	if fields == nil {
		return
	}

	cursor := c.surrounding.cursor
	for _, field := range fields.List {
		for _, name := range field.Names {
			if name.String() == "_" {
				continue
			}
			obj := c.pkg.GopTypesInfo().ObjectOf(name)
			if obj == nil {
				continue
			}

			// if we're in a field comment/doc, score that field as more relevant
			score := stdScore
			if field.Comment != nil && field.Comment.Pos() <= cursor && cursor <= field.Comment.End() {
				score = highScore
			} else if field.Doc != nil && field.Doc.Pos() <= cursor && cursor <= field.Doc.End() {
				score = highScore
			}

			c.deepState.enqueue(candidate{obj: obj, score: score})
		}
	}
}

func (c *gopCompleter) wantStructFieldCompletions() bool {
	clInfo := c.enclosingCompositeLiteral
	if clInfo == nil {
		return false
	}

	return clInfo.isStruct() && (clInfo.inKey || clInfo.maybeInFieldName)
}

func (c *gopCompleter) wantTypeName() bool {
	return !c.completionContext.commentCompletion && c.inference.typeName.wantTypeName
}

// gopObjChain decomposes e into a chain of objects if possible. For
// example, "foo.bar().baz" will yield []types.Object{foo, bar, baz}.
// If any part can't be turned into an object, return nil.
func gopObjChain(info *typesutil.Info, e ast.Expr) []types.Object {
	var objs []types.Object

	for e != nil {
		switch n := e.(type) {
		case *ast.Ident:
			obj := info.ObjectOf(n)
			if obj == nil {
				return nil
			}
			objs = append(objs, obj)
			e = nil
		case *ast.SelectorExpr:
			obj := info.ObjectOf(n.Sel)
			if obj == nil {
				return nil
			}
			objs = append(objs, obj)
			e = n.X
		case *ast.CallExpr:
			if len(n.Args) > 0 {
				return nil
			}
			e = n.Fun
		default:
			return nil
		}
	}

	// Reverse order so the layout matches the syntactic order.
	for i := 0; i < len(objs)/2; i++ {
		objs[i], objs[len(objs)-1-i] = objs[len(objs)-1-i], objs[i]
	}

	return objs
}

// selector finds completions for the specified selector expression.
func (c *gopCompleter) selector(ctx context.Context, sel *ast.SelectorExpr) error {
	c.inference.objChain = gopObjChain(c.pkg.GopTypesInfo(), sel.X)

	// True selector?
	tv, ok := c.pkg.GopTypesInfo().Types[sel.X]
	if goxls.DbgCompletion {
		log.Println("gopCompleter.selector:", sel.X, ok, "type:", tv.Type)
	}
	if ok {
		// goxls: assume tv.Addressable() => true
		// c.methodsAndFields(tv.Type, tv.Addressable(), nil, c.deepState.enqueue)
		c.methodsAndFields(tv.Type, true, nil, c.deepState.enqueue)
		if goxls.DbgCompletion {
			log.Println("gopCompleter methodsAndFields:", len(c.items))
		}
		c.addPostfixSnippetCandidates(ctx, sel)
		if goxls.DbgCompletion {
			log.Println("gopCompleter addPostfixSnippetCandidates:", len(c.items))
		}
		return nil
	}

	id, ok := sel.X.(*ast.Ident)
	if !ok {
		return nil
	}

	// Treat sel as a qualified identifier.
	var filter func(*source.Metadata) bool
	needImport := false
	if pkgName, ok := c.pkg.GopTypesInfo().Uses[id].(*types.PkgName); ok {
		// Qualified identifier with import declaration.
		imp := pkgName.Imported()

		// Known direct dependency? Expand using type information.
		if _, ok := c.pkg.Metadata().DepsByPkgPath[source.PackagePath(imp.Path())]; ok {
			c.packageMembers(imp, stdScore, nil, c.deepState.enqueue)
			return nil
		}

		// Imported declaration with missing type information.
		// Fall through to shallow completion of unimported package members.
		// Match candidate packages by path.
		filter = func(m *source.Metadata) bool {
			return strings.TrimPrefix(string(m.PkgPath), "vendor/") == imp.Path()
		}
	} else {
		// Qualified identifier without import declaration.
		// Match candidate packages by name.
		filter = func(m *source.Metadata) bool {
			return string(m.Name) == id.Name
		}
		needImport = true
	}

	// Search unimported packages.
	if !c.opts.unimported {
		return nil // feature disabled
	}

	// The deep completion algorithm is exceedingly complex and
	// deeply coupled to the now obsolete notions that all
	// token.Pos values can be interpreted by as a single FileSet
	// belonging to the Snapshot and that all types.Object values
	// are canonicalized by a single types.Importer mapping.
	// These invariants are no longer true now that gopls uses
	// an incremental approach, parsing and type-checking each
	// package separately.
	//
	// Consequently, completion of symbols defined in packages that
	// are not currently imported by the query file cannot use the
	// deep completion machinery which is based on type information.
	// Instead it must use only syntax information from a quick
	// parse of top-level declarations (but not function bodies).
	//
	// TODO(adonovan): rewrite the deep completion machinery to
	// not assume global Pos/Object realms and then use export
	// data instead of the quick parse approach taken here.

	// First, we search among packages in the forward transitive
	// closure of the workspace.
	// We'll use a fast parse to extract package members
	// from those that match the name/path criterion.
	all, err := c.snapshot.AllMetadata(ctx)
	if err != nil {
		return err
	}
	known := make(map[source.PackagePath]*source.Metadata)
	for _, m := range all {
		if m.Name == "main" {
			continue // not importable
		}
		if m.IsIntermediateTestVariant() {
			continue
		}
		// The only test variant we admit is "p [p.test]"
		// when we are completing within "p_test [p.test]",
		// as in that case we would like to offer completions
		// of the test variants' additional symbols.
		if m.ForTest != "" && c.pkg.Metadata().PkgPath != m.ForTest+"_test" {
			continue
		}
		if !filter(m) {
			continue
		}
		// Prefer previous entry unless this one is its test variant.
		if m.ForTest != "" || known[m.PkgPath] == nil {
			known[m.PkgPath] = m
		}
	}

	paths := make([]string, 0, len(known))
	for path := range known {
		paths = append(paths, string(path))
	}

	// Rank import paths as goimports would.
	var relevances map[string]float64
	if len(paths) > 0 {
		if err := c.snapshot.RunProcessEnvFunc(ctx, func(ctx context.Context, opts *imports.Options) error {
			var err error
			relevances, err = imports.ScoreImportPaths(ctx, opts.Env, paths)
			return err
		}); err != nil {
			return err
		}
		sort.Slice(paths, func(i, j int) bool {
			return relevances[paths[i]] > relevances[paths[j]]
		})
	}

	// quickParse does a quick parse of a single file of package m,
	// extracts exported package members and adds candidates to c.items.
	// TODO(rfindley): synchronizing access to c here does not feel right.
	// Consider adding a concurrency-safe API for completer.
	var cMu sync.Mutex // guards c.items and c.matcher
	var enough int32   // atomic bool
	quickParse := func(uri span.URI, m *source.Metadata) error {
		if atomic.LoadInt32(&enough) != 0 {
			return nil
		}

		fh, err := c.snapshot.ReadFile(ctx, uri)
		if err != nil {
			return err
		}
		content, err := fh.Content()
		if err != nil {
			return err
		}
		path := string(m.PkgPath)
		gopForEachPackageMember(content, func(tok token.Token, id *ast.Ident, fn *ast.FuncDecl) {
			if atomic.LoadInt32(&enough) != 0 {
				return
			}

			if !id.IsExported() {
				return
			}

			cMu.Lock()
			score := c.matcher.Score(id.Name)
			cMu.Unlock()

			if sel.Sel.Name != "_" && score == 0 {
				return // not a match; avoid constructing the completion item below
			}

			// The only detail is the kind and package: `var (from "example.com/foo")`
			// TODO(adonovan): pretty-print FuncDecl.FuncType or TypeSpec.Type?
			// TODO(adonovan): should this score consider the actual c.matcher.Score
			// of the item? How does this compare with the deepState.enqueue path?
			item := CompletionItem{
				Label:      id.Name,
				Detail:     fmt.Sprintf("%s (from %q)", strings.ToLower(tok.String()), m.PkgPath),
				InsertText: id.Name,
				Score:      unimportedScore(relevances[path]),
			}
			switch tok {
			case token.FUNC:
				item.Kind = protocol.FunctionCompletion
			case token.VAR:
				item.Kind = protocol.VariableCompletion
			case token.CONST:
				item.Kind = protocol.ConstantCompletion
			case token.TYPE:
				// Without types, we can't distinguish Class from Interface.
				item.Kind = protocol.ClassCompletion
			}

			if needImport {
				imp := &importInfo{importPath: path}
				if imports.ImportPathToAssumedName(path) != string(m.Name) {
					imp.name = string(m.Name)
				}
				item.AdditionalTextEdits, _ = c.importEdits(imp)
			}

			// For functions, add a parameter snippet.
			if fn != nil {
				var sn snippet.Builder
				sn.WriteText(id.Name)

				paramList := func(open, close string, list *ast.FieldList) {
					if list != nil {
						var cfg printer.Config // slight overkill
						var nparams int
						param := func(name string, typ ast.Expr) {
							if nparams > 0 {
								sn.WriteText(", ")
							}
							nparams++
							if c.opts.placeholders {
								sn.WritePlaceholder(func(b *snippet.Builder) {
									var buf strings.Builder
									buf.WriteString(name)
									buf.WriteByte(' ')
									cfg.Fprint(&buf, token.NewFileSet(), typ)
									b.WriteText(buf.String())
								})
							} else {
								sn.WriteText(name)
							}
						}

						sn.WriteText(open)
						for _, field := range list.List {
							if field.Names != nil {
								for _, name := range field.Names {
									param(name.Name, field.Type)
								}
							} else {
								param("_", field.Type)
							}
						}
						sn.WriteText(close)
					}
				}

				paramList("[", "]", typeparams.ForFuncType(fn.Type))
				paramList("(", ")", fn.Type.Params)

				item.snippet = &sn
			}

			cMu.Lock()
			c.items = append(c.items, item)
			// goxls func alias
			if tok == token.FUNC {
				if alias, ok := hasAliasName(id.Name); ok {
					var noSnip bool
					switch len(fn.Type.Params.List) {
					case 0:
						noSnip = true
					case 1:
						if fn.Recv != nil {
							if _, ok := fn.Type.Params.List[0].Type.(*ast.Ellipsis); ok {
								noSnip = true
							}
						}
					}
					c.items = append(c.items, cloneAliasItem(item, id.Name, alias, 0.0001, noSnip))
				}
			}
			if len(c.items) >= unimportedMemberTarget {
				atomic.StoreInt32(&enough, 1)
			}
			cMu.Unlock()
		})
		return nil
	}

	// Extract the package-level candidates using a quick parse.
	quickParseGo := c.quickParse(ctx, &cMu, &enough, sel.Sel.Name, relevances, needImport)
	var g errgroup.Group
	for _, path := range paths {
		m := known[source.PackagePath(path)]
		for _, uri := range m.CompiledGopFiles { // goxls: TODO - how to handle Go files?
			uri := uri
			g.Go(func() error {
				return quickParse(uri, m)
			})
		}
		for _, uri := range m.CompiledNongenGoFiles {
			uri := uri
			g.Go(func() error {
				return quickParseGo(uri, m)
			})
		}
	}
	if err := g.Wait(); err != nil {
		return err
	}

	// In addition, we search in the module cache using goimports.
	ctx, cancel := context.WithCancel(ctx)
	var mu sync.Mutex
	add := func(pkgExport imports.PackageExport) {
		mu.Lock()
		defer mu.Unlock()
		// TODO(adonovan): what if the actual package has a vendor/ prefix?
		if _, ok := known[source.PackagePath(pkgExport.Fix.StmtInfo.ImportPath)]; ok {
			return // We got this one above.
		}

		// Continue with untyped proposals.
		pkg := types.NewPackage(pkgExport.Fix.StmtInfo.ImportPath, pkgExport.Fix.IdentName)
		for _, export := range pkgExport.Exports {
			score := unimportedScore(pkgExport.Fix.Relevance)
			c.deepState.enqueue(candidate{
				obj:   types.NewVar(0, pkg, export, nil),
				score: score,
				imp: &importInfo{
					importPath: pkgExport.Fix.StmtInfo.ImportPath,
					name:       pkgExport.Fix.StmtInfo.Name,
				},
			})
		}
		if len(c.items) >= unimportedMemberTarget {
			cancel()
		}
	}

	c.completionCallbacks = append(c.completionCallbacks, func(ctx context.Context, opts *imports.Options) error {
		defer cancel()
		return imports.GetPackageExports(ctx, add, id.Name, c.filename, c.pkg.GetTypes().Name(), opts.Env)
	})
	return nil
}

func (c *gopCompleter) packageMembers(pkg *types.Package, score float64, imp *importInfo, cb func(candidate)) {
	scope := pkg.Scope()
	for _, name := range scope.Names() {
		obj := scope.Lookup(name)
		cb(candidate{
			obj:         obj,
			score:       score,
			imp:         imp,
			addressable: isVar(obj),
		})
	}
}

func (c *gopCompleter) methodsAndFields(typ types.Type, addressable bool, imp *importInfo, cb func(candidate)) {
	mset := c.methodSetCache[methodSetKey{typ, addressable}]
	if mset == nil {
		if addressable && !types.IsInterface(typ) && !isPointer(typ) {
			// Add methods of *T, which includes methods with receiver T.
			mset = types.NewMethodSet(types.NewPointer(typ))
		} else {
			// Add methods of T.
			mset = types.NewMethodSet(typ)
		}
		c.methodSetCache[methodSetKey{typ, addressable}] = mset
	}

	if isStarTestingDotF(typ) && addressable {
		// is that a sufficient test? (or is more care needed?)
		if c.fuzz(typ, mset, imp, cb, c.pkg.FileSet()) {
			return
		}
	}
	for i := 0; i < mset.Len(); i++ {
		cb(candidate{
			obj:         mset.At(i).Obj(),
			score:       stdScore,
			imp:         imp,
			addressable: addressable || isPointer(typ),
			lookup:      mset.Lookup,
		})
	}

	// Add fields of T.
	eachField(typ, func(v *types.Var) {
		cb(candidate{
			obj:         v,
			score:       stdScore - 0.01,
			imp:         imp,
			addressable: addressable || isPointer(typ),
		})
	})
}

// lexical finds completions in the lexical environment.
func (c *gopCompleter) lexical(ctx context.Context) error {
	var (
		builtinIota = types.Universe.Lookup("iota")
		builtinNil  = types.Universe.Lookup("nil")

		// TODO(rfindley): only allow "comparable" where it is valid (in constraint
		// position or embedded in interface declarations).
		// builtinComparable = types.Universe.Lookup("comparable")
	)
	classType, isClass := parserutil.GetClassType(c.file, c.filename)
	// Track seen variables to avoid showing completions for shadowed variables.
	// This works since we look at scopes from innermost to outermost.
	seen := make(map[string]struct{})

	// Process scopes innermost first.
	for i, scope := range c.scopes {
		if scope == nil {
			continue
		}

	Names:
		for _, name := range scope.Names() {
			declScope, obj := scope.LookupParent(name, c.pos)
			// Go+ class
			if isClass && name == classType {
				c.methodsAndFields(obj.Type(), true, nil, c.deepState.enqueue)
				continue
			}
			if declScope != scope {
				continue // Name was declared in some enclosing scope, or not at all.
			}

			// If obj's type is invalid, find the AST node that defines the lexical block
			// containing the declaration of obj. Don't resolve types for packages.
			if !isPkgName(obj) && !typeIsValid(obj.Type()) {
				// Match the scope to its ast.Node. If the scope is the package scope,
				// use the *ast.File as the starting node.
				var node ast.Node
				if i < len(c.path) {
					node = c.path[i]
				} else if i == len(c.path) { // use the *ast.File for package scope
					node = c.path[i-1]
				}
				if node != nil {
					if resolved := gopResolveInvalid(c.pkg.FileSet(), obj, node, c.pkg.GopTypesInfo()); resolved != nil {
						obj = resolved
					}
				}
			}

			// Don't use LHS of decl in RHS.
			for _, ident := range enclosingDeclLHS(c.path) {
				if obj.Pos() == ident.Pos() {
					continue Names
				}
			}

			// Don't suggest "iota" outside of const decls.
			if obj == builtinIota && !c.inConstDecl() {
				continue
			}

			// Rank outer scopes lower than inner.
			score := stdScore * math.Pow(.99, float64(i))

			// Dowrank "nil" a bit so it is ranked below more interesting candidates.
			if obj == builtinNil {
				score /= 2
			}

			// If we haven't already added a candidate for an object with this name.
			if _, ok := seen[obj.Name()]; !ok {
				seen[obj.Name()] = struct{}{}
				c.deepState.enqueue(candidate{
					obj:         obj,
					score:       score,
					addressable: isVar(obj),
				})
			}
		}
	}

	if c.inference.objType != nil {
		if named, _ := source.Deref(c.inference.objType).(*types.Named); named != nil {
			// If we expected a named type, check the type's package for
			// completion items. This is useful when the current file hasn't
			// imported the type's package yet.

			if named.Obj() != nil && named.Obj().Pkg() != nil {
				pkg := named.Obj().Pkg()

				// Make sure the package name isn't already in use by another
				// object, and that this file doesn't import the package yet.
				// TODO(adonovan): what if pkg.Path has vendor/ prefix?
				if _, ok := seen[pkg.Name()]; !ok && pkg != c.pkg.GetTypes() && !gopAlreadyImports(c.file, source.ImportPath(pkg.Path())) {
					seen[pkg.Name()] = struct{}{}
					obj := types.NewPkgName(0, nil, pkg.Name(), pkg)
					imp := &importInfo{
						importPath: pkg.Path(),
					}
					if imports.ImportPathToAssumedName(pkg.Path()) != pkg.Name() {
						imp.name = pkg.Name()
					}
					c.deepState.enqueue(candidate{
						obj:   obj,
						score: stdScore,
						imp:   imp,
					})
				}
			}
		}
	}

	if c.opts.unimported {
		if err := c.unimportedPackages(ctx, seen); err != nil {
			return err
		}
	}

	if c.inference.typeName.isTypeParam {
		// If we are completing a type param, offer each structural type.
		// This ensures we suggest "[]int" and "[]float64" for a constraint
		// with type union "[]int | []float64".
		if t, _ := c.inference.objType.(*types.Interface); t != nil {
			terms, _ := typeparams.InterfaceTermSet(t)
			for _, term := range terms {
				c.injectType(ctx, term.Type())
			}
		}
	} else {
		c.injectType(ctx, c.inference.objType)
	}

	// Add keyword completion items appropriate in the current context.
	c.addKeywordCompletions()

	return nil
}

// gopAlreadyImports reports whether f has an import with the specified path.
func gopAlreadyImports(f *ast.File, path source.ImportPath) bool {
	for _, s := range f.Imports {
		if source.GopUnquoteImportPath(s) == path {
			return true
		}
	}
	return false
}

// injectType manufactures candidates based on the given type. This is
// intended for types not discoverable via lexical search, such as
// composite and/or generic types. For example, if the type is "[]int",
// this method makes sure you get candidates "[]int{}" and "[]int"
// (the latter applies when completing a type name).
func (c *gopCompleter) injectType(ctx context.Context, t types.Type) {
	if t == nil {
		return
	}

	t = source.Deref(t)

	// If we have an expected type and it is _not_ a named type, handle
	// it specially. Non-named types like "[]int" will never be
	// considered via a lexical search, so we need to directly inject
	// them. Also allow generic types since lexical search does not
	// infer instantiated versions of them.
	if named, _ := t.(*types.Named); named == nil || typeparams.ForNamed(named).Len() > 0 {
		// If our expected type is "[]int", this will add a literal
		// candidate of "[]int{}".
		c.literal(ctx, t, nil)

		if _, isBasic := t.(*types.Basic); !isBasic {
			// If we expect a non-basic type name (e.g. "[]int"), hack up
			// a named type whose name is literally "[]int". This allows
			// us to reuse our object based completion machinery.
			fakeNamedType := candidate{
				obj:   types.NewTypeName(token.NoPos, nil, types.TypeString(t, c.qf), t),
				score: stdScore,
			}
			// Make sure the type name matches before considering
			// candidate. This cuts down on useless candidates.
			if c.matchingTypeName(&fakeNamedType) {
				c.deepState.enqueue(fakeNamedType)
			}
		}
	}
}

func (c *gopCompleter) unimportedPackages(ctx context.Context, seen map[string]struct{}) error {
	var prefix string
	if c.surrounding != nil {
		prefix = c.surrounding.Prefix()
	}

	// Don't suggest unimported packages if we have absolutely nothing
	// to go on.
	if prefix == "" {
		return nil
	}

	count := 0

	// Search the forward transitive closure of the workspace.
	all, err := c.snapshot.AllMetadata(ctx)
	if err != nil {
		return err
	}
	pkgNameByPath := make(map[source.PackagePath]string)
	var paths []string // actually PackagePaths
	for _, m := range all {
		if m.ForTest != "" {
			continue // skip all test variants
		}
		if m.Name == "main" {
			continue // main is non-importable
		}
		if !strings.HasPrefix(string(m.Name), prefix) {
			continue // not a match
		}
		paths = append(paths, string(m.PkgPath))
		pkgNameByPath[m.PkgPath] = string(m.Name)
	}

	// Rank candidates using goimports' algorithm.
	var relevances map[string]float64
	if len(paths) != 0 {
		if err := c.snapshot.RunProcessEnvFunc(ctx, func(ctx context.Context, opts *imports.Options) error {
			var err error
			relevances, err = imports.ScoreImportPaths(ctx, opts.Env, paths)
			return err
		}); err != nil {
			return err
		}
	}
	sort.Slice(paths, func(i, j int) bool {
		if relevances[paths[i]] != relevances[paths[j]] {
			return relevances[paths[i]] > relevances[paths[j]]
		}

		// Fall back to lexical sort to keep truncated set of candidates
		// in a consistent order.
		return paths[i] < paths[j]
	})

	for _, path := range paths {
		name := pkgNameByPath[source.PackagePath(path)]
		if _, ok := seen[name]; ok {
			continue
		}
		imp := &importInfo{
			importPath: path,
		}
		if imports.ImportPathToAssumedName(path) != name {
			imp.name = name
		}
		if count >= maxUnimportedPackageNames {
			return nil
		}
		c.deepState.enqueue(candidate{
			// Pass an empty *types.Package to disable deep completions.
			obj:   types.NewPkgName(0, nil, name, types.NewPackage(path, name)),
			score: unimportedScore(relevances[path]),
			imp:   imp,
		})
		count++
	}

	ctx, cancel := context.WithCancel(ctx)
	_ = ctx

	var mu sync.Mutex
	add := func(pkg imports.ImportFix) {
		mu.Lock()
		defer mu.Unlock()
		if _, ok := seen[pkg.IdentName]; ok {
			return
		}
		if _, ok := relevances[pkg.StmtInfo.ImportPath]; ok {
			return
		}

		if count >= maxUnimportedPackageNames {
			cancel()
			return
		}

		// Do not add the unimported packages to seen, since we can have
		// multiple packages of the same name as completion suggestions, since
		// only one will be chosen.
		obj := types.NewPkgName(0, nil, pkg.IdentName, types.NewPackage(pkg.StmtInfo.ImportPath, pkg.IdentName))
		c.deepState.enqueue(candidate{
			obj:   obj,
			score: unimportedScore(pkg.Relevance),
			imp: &importInfo{
				importPath: pkg.StmtInfo.ImportPath,
				name:       pkg.StmtInfo.Name,
			},
		})
		count++
	}
	c.completionCallbacks = append(c.completionCallbacks, func(ctx context.Context, opts *imports.Options) error {
		defer cancel()
		return imports.GetAllCandidates(ctx, add, prefix, c.filename, c.pkg.GetTypes().Name(), opts.Env)
	})
	return nil
}

func (c *gopCompleter) inConstDecl() bool {
	for _, n := range c.path {
		if decl, ok := n.(*ast.GenDecl); ok && decl.Tok == token.CONST {
			return true
		}
	}
	return false
}

// innermostScope returns the innermost scope for c.pos.
func (c *gopCompleter) innermostScope() *types.Scope {
	for _, s := range c.scopes {
		if s != nil {
			return s
		}
	}
	return nil
}

// structLiteralFieldName finds completions for struct field names inside a struct literal.
func (c *gopCompleter) structLiteralFieldName(ctx context.Context) error {
	clInfo := c.enclosingCompositeLiteral

	// Mark fields of the composite literal that have already been set,
	// except for the current field.
	addedFields := make(map[*types.Var]bool)
	for _, el := range clInfo.cl.Elts {
		if kvExpr, ok := el.(*ast.KeyValueExpr); ok {
			if clInfo.kv == kvExpr {
				continue
			}

			if key, ok := kvExpr.Key.(*ast.Ident); ok {
				if used, ok := c.pkg.GopTypesInfo().Uses[key]; ok {
					if usedVar, ok := used.(*types.Var); ok {
						addedFields[usedVar] = true
					}
				}
			}
		}
	}

	deltaScore := 0.0001
	switch t := clInfo.clType.(type) {
	case *types.Struct:
		for i := 0; i < t.NumFields(); i++ {
			field := t.Field(i)
			if !addedFields[field] {
				c.deepState.enqueue(candidate{
					obj:   field,
					score: highScore - float64(i)*deltaScore,
				})
			}
		}

		// Add lexical completions if we aren't certain we are in the key part of a
		// key-value pair.
		if clInfo.maybeInFieldName {
			return c.lexical(ctx)
		}
	default:
		return c.lexical(ctx)
	}

	return nil
}

func (cl *gopCompLitInfo) isStruct() bool {
	_, ok := cl.clType.(*types.Struct)
	return ok
}

func (c *gopCompleter) expectedCompositeLiteralType() types.Type {
	clInfo := c.enclosingCompositeLiteral
	switch t := clInfo.clType.(type) {
	case *types.Slice:
		if clInfo.inKey {
			return types.Typ[types.UntypedInt]
		}
		return t.Elem()
	case *types.Array:
		if clInfo.inKey {
			return types.Typ[types.UntypedInt]
		}
		return t.Elem()
	case *types.Map:
		if clInfo.inKey {
			return t.Key()
		}
		return t.Elem()
	case *types.Struct:
		// If we are completing a key (i.e. field name), there is no expected type.
		if clInfo.inKey {
			return nil
		}

		// If we are in a key-value pair, but not in the key, then we must be on the
		// value side. The expected type of the value will be determined from the key.
		if clInfo.kv != nil {
			if key, ok := clInfo.kv.Key.(*ast.Ident); ok {
				for i := 0; i < t.NumFields(); i++ {
					if field := t.Field(i); field.Name() == key.Name {
						return field.Type()
					}
				}
			}
		} else {
			// If we aren't in a key-value pair and aren't in the key, we must be using
			// implicit field names.

			// The order of the literal fields must match the order in the struct definition.
			// Find the element that the position belongs to and suggest that field's type.
			if i := gopExprAtPos(c.pos, clInfo.cl.Elts); i < t.NumFields() {
				return t.Field(i).Type()
			}
		}
	}
	return nil
}

func (c *gopCompleter) expectedCallParamType(inf candidateInference, node *ast.CallExpr, sig *types.Signature) candidateInference {
	numParams := sig.Params().Len()
	if numParams == 0 {
		return inf
	}

	exprIdx := gopExprAtPos(c.pos, node.Args)

	// If we have one or zero arg expressions, we may be
	// completing to a function call that returns multiple
	// values, in turn getting passed in to the surrounding
	// call. Record the assignees so we can favor function
	// calls that return matching values.
	if len(node.Args) <= 1 && exprIdx == 0 {
		for i := 0; i < sig.Params().Len(); i++ {
			inf.assignees = append(inf.assignees, sig.Params().At(i).Type())
		}

		// Record that we may be completing into variadic parameters.
		inf.variadicAssignees = sig.Variadic()
	}

	// Make sure not to run past the end of expected parameters.
	if exprIdx >= numParams {
		inf.objType = sig.Params().At(numParams - 1).Type()
	} else {
		inf.objType = sig.Params().At(exprIdx).Type()
	}

	if sig.Variadic() && exprIdx >= (numParams-1) {
		// If we are completing a variadic param, deslice the variadic type.
		inf.objType = deslice(inf.objType)
		// Record whether we are completing the initial variadic param.
		inf.variadic = exprIdx == numParams-1 && len(node.Args) <= numParams

		// Check if we can infer object kind from printf verb.
		inf.objKind |= gopPrintfArgKind(c.pkg.GopTypesInfo(), node, exprIdx)
	}

	// If our expected type is an uninstantiated generic type param,
	// swap to the constraint which will do a decent job filtering
	// candidates.
	if tp, _ := inf.objType.(*typeparams.TypeParam); tp != nil {
		inf.objType = tp.Constraint()
	}

	return inf
}

// matchingCandidate reports whether cand matches our type inferences.
// It mutates cand's score in certain cases.
func (c *gopCompleter) matchingCandidate(cand *candidate) bool {
	if c.completionContext.commentCompletion {
		return false
	}

	// Bail out early if we are completing a field name in a composite literal.
	if v, ok := cand.obj.(*types.Var); ok && v.IsField() && c.wantStructFieldCompletions() {
		return true
	}

	if isTypeName(cand.obj) {
		return c.matchingTypeName(cand)
	} else if c.wantTypeName() {
		// If we want a type, a non-type object never matches.
		return false
	}

	if c.inference.candTypeMatches(cand) {
		return true
	}

	candType := cand.obj.Type()
	if candType == nil {
		return false
	}

	if sig, ok := candType.Underlying().(*types.Signature); ok {
		if c.inference.assigneesMatch(cand, sig) {
			// Invoke the candidate if its results are multi-assignable.
			cand.mods = append(cand.mods, invoke)
			return true
		}
	}

	// Default to invoking *types.Func candidates. This is so function
	// completions in an empty statement (or other cases with no expected type)
	// are invoked by default.
	if isFunc(cand.obj) {
		cand.mods = append(cand.mods, invoke)
	}

	return false
}

func (c *gopCompleter) matchingTypeName(cand *candidate) bool {
	if !c.wantTypeName() {
		return false
	}

	typeMatches := func(candType types.Type) bool {
		// Take into account any type name modifier prefixes.
		candType = c.inference.applyTypeNameModifiers(candType)

		if from := c.inference.typeName.assertableFrom; from != nil {
			// Don't suggest the starting type in type assertions. For example,
			// if "foo" is an io.Writer, don't suggest "foo.(io.Writer)".
			if types.Identical(from, candType) {
				return false
			}

			if intf, ok := from.Underlying().(*types.Interface); ok {
				if !types.AssertableTo(intf, candType) {
					return false
				}
			}
		}

		if c.inference.typeName.wantComparable && !types.Comparable(candType) {
			return false
		}

		// Skip this type if it has already been used in another type
		// switch case.
		for _, seen := range c.inference.typeName.seenTypeSwitchCases {
			if types.Identical(candType, seen) {
				return false
			}
		}

		// We can expect a type name and have an expected type in cases like:
		//
		//   var foo []int
		//   foo = []i<>
		//
		// Where our expected type is "[]int", and we expect a type name.
		if c.inference.objType != nil {
			return assignableTo(candType, c.inference.objType)
		}

		// Default to saying any type name is a match.
		return true
	}

	t := cand.obj.Type()

	if typeMatches(t) {
		return true
	}

	if !types.IsInterface(t) && typeMatches(types.NewPointer(t)) {
		if c.inference.typeName.compLitType {
			// If we are completing a composite literal type as in
			// "foo<>{}", to make a pointer we must prepend "&".
			cand.mods = append(cand.mods, reference)
		} else {
			// If we are completing a normal type name such as "foo<>", to
			// make a pointer we must prepend "*".
			cand.mods = append(cand.mods, dereference)
		}
		return true
	}

	return false
}

func (c *gopCompleter) setSurrounding(ident *ast.Ident) {
	if c.surrounding != nil {
		return
	}
	if !(ident.Pos() <= c.pos && c.pos <= ident.End()) {
		return
	}

	c.surrounding = &Selection{
		content: ident.Name,
		cursor:  c.pos,
		// Overwrite the prefix only.
		tokFile: c.tokFile,
		start:   ident.Pos(),
		end:     ident.End(),
		mapper:  c.mapper,
	}

	c.setMatcherFromPrefix(c.surrounding.Prefix())
}

func (c *gopCompleter) setMatcherFromPrefix(prefix string) {
	switch c.opts.matcher {
	case source.Fuzzy:
		c.matcher = fuzzy.NewMatcher(prefix)
	case source.CaseSensitive:
		c.matcher = prefixMatcher(prefix)
	default:
		c.matcher = insensitivePrefixMatcher(strings.ToLower(prefix))
	}
}

// gopFindSwitchStmt returns an *ast.CaseClause's corresponding *ast.SwitchStmt or
// *ast.TypeSwitchStmt. path should start from the case clause's first ancestor.
func gopFindSwitchStmt(path []ast.Node, pos token.Pos, c *ast.CaseClause) ast.Stmt {
	// Make sure position falls within a "case <>:" clause.
	if gopExprAtPos(pos, c.List) >= len(c.List) {
		return nil
	}
	// A case clause is always nested within a block statement in a switch statement.
	if len(path) < 2 {
		return nil
	}
	if _, ok := path[0].(*ast.BlockStmt); !ok {
		return nil
	}
	switch s := path[1].(type) {
	case *ast.SwitchStmt:
		return s
	case *ast.TypeSwitchStmt:
		return s
	default:
		return nil
	}
}

// gopExpectTypeName returns information about the expected type name at position.
func gopExpectTypeName(c *gopCompleter) typeNameInference {
	var inf typeNameInference

Nodes:
	for i, p := range c.path {
		switch n := p.(type) {
		case *ast.FieldList:
			// Expect a type name if pos is in a FieldList. This applies to
			// FuncType params/results, FuncDecl receiver, StructType, and
			// InterfaceType. We don't need to worry about the field name
			// because completion bails out early if pos is in an *ast.Ident
			// that defines an object.
			inf.wantTypeName = true
			break Nodes
		case *ast.CaseClause:
			// Expect type names in type switch case clauses.
			if swtch, ok := gopFindSwitchStmt(c.path[i+1:], c.pos, n).(*ast.TypeSwitchStmt); ok {
				// The case clause types must be assertable from the type switch parameter.
				ast.Inspect(swtch.Assign, func(n ast.Node) bool {
					if ta, ok := n.(*ast.TypeAssertExpr); ok {
						inf.assertableFrom = c.pkg.GopTypesInfo().TypeOf(ta.X)
						return false
					}
					return true
				})
				inf.wantTypeName = true

				// Track the types that have already been used in this
				// switch's case statements so we don't recommend them.
				for _, e := range swtch.Body.List {
					for _, typeExpr := range e.(*ast.CaseClause).List {
						// Skip if type expression contains pos. We don't want to
						// count it as already used if the user is completing it.
						if typeExpr.Pos() < c.pos && c.pos <= typeExpr.End() {
							continue
						}

						if t := c.pkg.GopTypesInfo().TypeOf(typeExpr); t != nil {
							inf.seenTypeSwitchCases = append(inf.seenTypeSwitchCases, t)
						}
					}
				}

				break Nodes
			}
			return typeNameInference{}
		case *ast.TypeAssertExpr:
			// Expect type names in type assert expressions.
			if n.Lparen < c.pos && c.pos <= n.Rparen {
				// The type in parens must be assertable from the expression type.
				inf.assertableFrom = c.pkg.GopTypesInfo().TypeOf(n.X)
				inf.wantTypeName = true
				break Nodes
			}
			return typeNameInference{}
		case *ast.StarExpr:
			inf.modifiers = append(inf.modifiers, typeMod{mod: reference})
		case *ast.CompositeLit:
			// We want a type name if position is in the "Type" part of a
			// composite literal (e.g. "Foo<>{}").
			if n.Type != nil && n.Type.Pos() <= c.pos && c.pos <= n.Type.End() {
				inf.wantTypeName = true
				inf.compLitType = true

				if i < len(c.path)-1 {
					// Track preceding "&" operator. Technically it applies to
					// the composite literal and not the type name, but if
					// affects our type completion nonetheless.
					if u, ok := c.path[i+1].(*ast.UnaryExpr); ok && u.Op == token.AND {
						inf.modifiers = append(inf.modifiers, typeMod{mod: reference})
					}
				}
			}
			break Nodes
		case *ast.ArrayType:
			// If we are inside the "Elt" part of an array type, we want a type name.
			if n.Elt.Pos() <= c.pos && c.pos <= n.Elt.End() {
				inf.wantTypeName = true
				if n.Len == nil {
					// No "Len" expression means a slice type.
					inf.modifiers = append(inf.modifiers, typeMod{mod: sliceType})
				} else {
					// Try to get the array type using the constant value of "Len".
					tv, ok := c.pkg.GopTypesInfo().Types[n.Len]
					if ok && tv.Value != nil && tv.Value.Kind() == constant.Int {
						if arrayLen, ok := constant.Int64Val(tv.Value); ok {
							inf.modifiers = append(inf.modifiers, typeMod{mod: arrayType, arrayLen: arrayLen})
						}
					}
				}

				// ArrayTypes can be nested, so keep going if our parent is an
				// ArrayType.
				if i < len(c.path)-1 {
					if _, ok := c.path[i+1].(*ast.ArrayType); ok {
						continue Nodes
					}
				}

				break Nodes
			}
		case *ast.MapType:
			inf.wantTypeName = true
			if n.Key != nil {
				inf.wantComparable = source.NodeContains(n.Key, c.pos)
			} else {
				// If the key is empty, assume we are completing the key if
				// pos is directly after the "map[".
				inf.wantComparable = c.pos == n.Pos()+token.Pos(len("map["))
			}
			break Nodes
		case *ast.ValueSpec:
			inf.wantTypeName = source.NodeContains(n.Type, c.pos)
			break Nodes
		case *ast.TypeSpec:
			inf.wantTypeName = source.NodeContains(n.Type, c.pos)
		default:
			if breaksExpectedTypeInference(p, c.pos) {
				return typeNameInference{}
			}
		}
	}

	return inf
}

// gopExpectedCandidate returns information about the expected candidate
// for an expression at the query position.
func gopExpectedCandidate(ctx context.Context, c *gopCompleter) (inf candidateInference) {
	inf.typeName = gopExpectTypeName(c)

	if c.enclosingCompositeLiteral != nil {
		inf.objType = c.expectedCompositeLiteralType()
	}

Nodes:
	for i, node := range c.path {
		switch node := node.(type) {
		case *ast.BinaryExpr:
			// Determine if query position comes from left or right of op.
			e := node.X
			if c.pos < node.OpPos {
				e = node.Y
			}
			if tv, ok := c.pkg.GopTypesInfo().Types[e]; ok {
				switch node.Op {
				case token.LAND, token.LOR:
					// Don't infer "bool" type for "&&" or "||". Often you want
					// to compose a boolean expression from non-boolean
					// candidates.
				default:
					inf.objType = tv.Type
				}
				break Nodes
			}
		case *ast.AssignStmt:
			// Only rank completions if you are on the right side of the token.
			if c.pos > node.TokPos {
				i := gopExprAtPos(c.pos, node.Rhs)
				if i >= len(node.Lhs) {
					i = len(node.Lhs) - 1
				}
				if tv, ok := c.pkg.GopTypesInfo().Types[node.Lhs[i]]; ok {
					inf.objType = tv.Type
				}

				// If we have a single expression on the RHS, record the LHS
				// assignees so we can favor multi-return function calls with
				// matching result values.
				if len(node.Rhs) <= 1 {
					for _, lhs := range node.Lhs {
						inf.assignees = append(inf.assignees, c.pkg.GopTypesInfo().TypeOf(lhs))
					}
				} else {
					// Otherwise, record our single assignee, even if its type is
					// not available. We use this info to downrank functions
					// with the wrong number of result values.
					inf.assignees = append(inf.assignees, c.pkg.GopTypesInfo().TypeOf(node.Lhs[i]))
				}
			}
			return inf
		case *ast.ValueSpec:
			if node.Type != nil && c.pos > node.Type.End() {
				inf.objType = c.pkg.GopTypesInfo().TypeOf(node.Type)
			}
			return inf
		case *ast.CallExpr:
			// Only consider CallExpr args if position falls between parens.
			if node.Lparen < c.pos && c.pos <= node.Rparen {
				// For type conversions like "int64(foo)" we can only infer our
				// desired type is convertible to int64.
				if typ := gopTypeConversion(node, c.pkg.GopTypesInfo()); typ != nil {
					inf.convertibleTo = typ
					break Nodes
				}

				sig, _ := c.pkg.GopTypesInfo().Types[node.Fun].Type.(*types.Signature)

				if sig != nil && typeparams.ForSignature(sig).Len() > 0 {
					// If we are completing a generic func call, re-check the call expression.
					// This allows type param inference to work in cases like:
					//
					// func foo[T any](T) {}
					// foo[int](<>) // <- get "int" completions instead of "T"
					//
					// TODO: remove this after https://go.dev/issue/52503
					// TODO: goxls not implement typesutil.CheckExpr
					// info := &typesutil.Info{Types: make(map[ast.Expr]types.TypeAndValue)}
					// typesutil.CheckExpr(c.pkg.FileSet(), c.pkg.GetTypes(), node.Fun.Pos(), node.Fun, info)
					// sig, _ = info.Types[node.Fun].Type.(*types.Signature)
				}

				if sig != nil {
					inf = c.expectedCallParamType(inf, node, sig)
				}

				if funIdent, ok := node.Fun.(*ast.Ident); ok {
					obj := c.pkg.GopTypesInfo().ObjectOf(funIdent)

					if obj != nil && obj.Parent() == types.Universe {
						// Defer call to builtinArgType so we can provide it the
						// inferred type from its parent node.
						defer func() {
							inf = c.builtinArgType(obj, node, inf)
							inf.objKind = c.builtinArgKind(ctx, obj, node)
						}()

						// The expected type of builtin arguments like append() is
						// the expected type of the builtin call itself. For
						// example:
						//
						// var foo []int = append(<>)
						//
						// To find the expected type at <> we "skip" the append()
						// node and get the expected type one level up, which is
						// []int.
						continue Nodes
					}
				}

				return inf
			}
		case *ast.ReturnStmt:
			if c.enclosingFunc != nil {
				sig := c.enclosingFunc.sig
				// Find signature result that corresponds to our return statement.
				if resultIdx := gopExprAtPos(c.pos, node.Results); resultIdx < len(node.Results) {
					if resultIdx < sig.Results().Len() {
						inf.objType = sig.Results().At(resultIdx).Type()
					}
				}
			}
			return inf
		case *ast.CaseClause:
			if swtch, ok := gopFindSwitchStmt(c.path[i+1:], c.pos, node).(*ast.SwitchStmt); ok {
				if tv, ok := c.pkg.GopTypesInfo().Types[swtch.Tag]; ok {
					inf.objType = tv.Type

					// Record which objects have already been used in the case
					// statements so we don't suggest them again.
					for _, cc := range swtch.Body.List {
						for _, caseExpr := range cc.(*ast.CaseClause).List {
							// Don't record the expression we are currently completing.
							if caseExpr.Pos() < c.pos && c.pos <= caseExpr.End() {
								continue
							}

							if objs := gopObjChain(c.pkg.GopTypesInfo(), caseExpr); len(objs) > 0 {
								inf.penalized = append(inf.penalized, penalizedObj{objChain: objs, penalty: 0.1})
							}
						}
					}
				}
			}
			return inf
		case *ast.SliceExpr:
			// Make sure position falls within the brackets (e.g. "foo[a:<>]").
			if node.Lbrack < c.pos && c.pos <= node.Rbrack {
				inf.objType = types.Typ[types.UntypedInt]
			}
			return inf
		case *ast.IndexExpr:
			// Make sure position falls within the brackets (e.g. "foo[<>]").
			if node.Lbrack < c.pos && c.pos <= node.Rbrack {
				if tv, ok := c.pkg.GopTypesInfo().Types[node.X]; ok {
					switch t := tv.Type.Underlying().(type) {
					case *types.Map:
						inf.objType = t.Key()
					case *types.Slice, *types.Array:
						inf.objType = types.Typ[types.UntypedInt]
					}

					if ct := expectedConstraint(tv.Type, 0); ct != nil {
						inf.objType = ct
						inf.typeName.wantTypeName = true
						inf.typeName.isTypeParam = true
					}
				}
			}
			return inf
		case *typeparams.IndexListExpr:
			if node.Lbrack < c.pos && c.pos <= node.Rbrack {
				if tv, ok := c.pkg.GopTypesInfo().Types[node.X]; ok {
					if ct := expectedConstraint(tv.Type, gopExprAtPos(c.pos, node.Indices)); ct != nil {
						inf.objType = ct
						inf.typeName.wantTypeName = true
						inf.typeName.isTypeParam = true
					}
				}
			}
			return inf
		case *ast.SendStmt:
			// Make sure we are on right side of arrow (e.g. "foo <- <>").
			if c.pos > node.Arrow+1 {
				if tv, ok := c.pkg.GopTypesInfo().Types[node.Chan]; ok {
					if ch, ok := tv.Type.Underlying().(*types.Chan); ok {
						inf.objType = ch.Elem()
					}
				}
			}
			return inf
		case *ast.RangeStmt:
			if source.NodeContains(node.X, c.pos) {
				inf.objKind |= kindSlice | kindArray | kindMap | kindString
				if node.Value == nil {
					inf.objKind |= kindChan
				}
			}
			return inf
		case *ast.StarExpr:
			inf.modifiers = append(inf.modifiers, typeMod{mod: dereference})
		case *ast.UnaryExpr:
			switch node.Op {
			case token.AND:
				inf.modifiers = append(inf.modifiers, typeMod{mod: reference})
			case token.ARROW:
				inf.modifiers = append(inf.modifiers, typeMod{mod: chanRead})
			}
		case *ast.DeferStmt, *ast.GoStmt:
			inf.objKind |= kindFunc
			return inf
		default:
			if breaksExpectedTypeInference(node, c.pos) {
				return inf
			}
		}
	}

	return inf
}

// gopForEachPackageMember calls f(tok, id, fn) for each package-level
// TYPE/VAR/CONST/FUNC declaration in the Go source file, based on a
// quick partial parse. fn is non-nil only for function declarations.
// The AST position information is garbage.
func gopForEachPackageMember(content []byte, f func(tok token.Token, id *ast.Ident, fn *ast.FuncDecl)) {
	purged := goxlsastutil.PurgeFuncBodies(content)
	file, _ := parserutil.ParseFile(token.NewFileSet(), "", purged, 0)
	for _, decl := range file.Decls {
		switch decl := decl.(type) {
		case *ast.GenDecl:
			for _, spec := range decl.Specs {
				switch spec := spec.(type) {
				case *ast.ValueSpec: // var/const
					for _, id := range spec.Names {
						f(decl.Tok, id, nil)
					}
				case *ast.TypeSpec:
					f(decl.Tok, spec.Name, nil)
				}
			}
		case *ast.FuncDecl:
			if decl.Recv == nil {
				f(token.FUNC, decl.Name, decl)
			}
		}
	}
}
