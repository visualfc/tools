package source

import (
	"context"
	"go/ast"
	"go/doc"
	"go/types"

	"golang.org/x/tools/gopls/internal/bug"
	"golang.org/x/tools/internal/typeparams"
)

// GopNewSignature returns formatted signature for a types.Signature struct.
func GopNewSignature(ctx context.Context, s Snapshot, pkg Package, sig *types.Signature, comment *ast.CommentGroup, qf types.Qualifier, mq MetadataQualifier) (*signature, error) {
	var tparams []string
	tpList := typeparams.ForSignature(sig)
	for i := 0; i < tpList.Len(); i++ {
		tparam := tpList.At(i)
		// TODO: is it possible to reuse the logic from FormatVarType here?
		s := tparam.Obj().Name() + " " + tparam.Constraint().String()
		tparams = append(tparams, s)
	}

	params := make([]string, 0, sig.Params().Len())
	for i := 0; i < sig.Params().Len(); i++ {
		el := sig.Params().At(i)
		typ, err := GopFormatVarType(ctx, s, pkg, el, qf, mq)
		if err != nil {
			return nil, err
		}
		p := typ
		if el.Name() != "" {
			p = el.Name() + " " + typ
		}
		params = append(params, p)
	}

	var needResultParens bool
	results := make([]string, 0, sig.Results().Len())
	for i := 0; i < sig.Results().Len(); i++ {
		if i >= 1 {
			needResultParens = true
		}
		el := sig.Results().At(i)
		typ, err := GopFormatVarType(ctx, s, pkg, el, qf, mq)
		if err != nil {
			return nil, err
		}
		if el.Name() == "" {
			results = append(results, typ)
		} else {
			if i == 0 {
				needResultParens = true
			}
			results = append(results, el.Name()+" "+typ)
		}
	}
	var d string
	if comment != nil {
		d = comment.Text()
	}
	switch s.View().Options().HoverKind {
	case SynopsisDocumentation:
		d = doc.Synopsis(d)
	case NoDocumentation:
		d = ""
	}
	return &signature{
		doc:              d,
		typeParams:       tparams,
		params:           params,
		results:          results,
		variadic:         sig.Variadic(),
		needResultParens: needResultParens,
	}, nil
}

// GopFormatVarType formats a *types.Var, accounting for type aliases.
// To do this, it looks in the AST of the file in which the object is declared.
// On any errors, it always falls back to types.TypeString.
//
// TODO(rfindley): this function could return the actual name used in syntax,
// for better parameter names.
func GopFormatVarType(ctx context.Context, snapshot Snapshot, srcpkg Package, obj *types.Var, qf types.Qualifier, mq MetadataQualifier) (string, error) {
	// TODO(rfindley): This looks wrong. The previous comment said:
	// "If the given expr refers to a type parameter, then use the
	// object's Type instead of the type parameter declaration. This helps
	// format the instantiated type as opposed to the original undeclared
	// generic type".
	//
	// But of course, if obj is a type param, we are formatting a generic type
	// and not an instantiated type. Handling for instantiated types must be done
	// at a higher level.
	//
	// Left this during refactoring in order to preserve pre-existing logic.
	if typeparams.IsTypeParam(obj.Type()) {
		return types.TypeString(obj.Type(), qf), nil
	}

	if obj.Pkg() == nil || !obj.Pos().IsValid() {
		// This is defensive, though it is extremely unlikely we'll ever have a
		// builtin var.
		return types.TypeString(obj.Type(), qf), nil
	}

	// TODO(rfindley): parsing to produce candidates can be costly; consider
	// using faster methods.
	targetpgf, pos, err := parseFull(ctx, snapshot, srcpkg.FileSet(), obj.Pos())
	if err != nil {
		return "", err // e.g. ctx cancelled
	}

	// why we should find file in deps and why we can't find it?
	targetMeta := gopFindFileInDeps(snapshot, srcpkg.Metadata(), targetpgf.URI)
	if targetMeta == nil {
		// If we have an object from type-checking, it should exist in a file in
		// the forward transitive closure.
		return "", bug.Errorf("failed to find file %q in deps of %q", targetpgf.URI, srcpkg.Metadata().ID)
	}

	decl, spec, field := findDeclInfo([]*ast.File{targetpgf.File}, pos)

	// We can't handle type parameters correctly, so we fall back on TypeString
	// for parameterized decls.
	if decl, _ := decl.(*ast.FuncDecl); decl != nil {
		if typeparams.ForFuncType(decl.Type).NumFields() > 0 {
			return types.TypeString(obj.Type(), qf), nil // in generic function
		}
		if decl.Recv != nil && len(decl.Recv.List) > 0 {
			if x, _, _, _ := typeparams.UnpackIndexExpr(decl.Recv.List[0].Type); x != nil {
				return types.TypeString(obj.Type(), qf), nil // in method of generic type
			}
		}
	}
	if spec, _ := spec.(*ast.TypeSpec); spec != nil && typeparams.ForTypeSpec(spec).NumFields() > 0 {
		return types.TypeString(obj.Type(), qf), nil // in generic type decl
	}

	if field == nil {
		// TODO(rfindley): we should never reach here from an ordinary var, so
		// should probably return an error here.
		return types.TypeString(obj.Type(), qf), nil
	}
	expr := field.Type

	rq := requalifier(snapshot, targetpgf.File, targetMeta, mq)

	// The type names in the AST may not be correctly qualified.
	// Determine the package name to use based on the package that originated
	// the query and the package in which the type is declared.
	// We then qualify the value by cloning the AST node and editing it.
	expr = qualifyTypeExpr(expr, rq)

	// If the request came from a different package than the one in which the
	// types are defined, we may need to modify the qualifiers.
	return FormatNodeFile(targetpgf.Tok, expr), nil
}
