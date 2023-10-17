// Copyright 2023 The GoPlus Authors (goplus.org). All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package source

import (
	"context"

	"golang.org/x/tools/gopls/internal/span"
)

/*
import (
	"go/types"
	"strconv"
	"strings"

	"github.com/goplus/gop/ast"
	"github.com/goplus/gop/token"
	"golang.org/x/tools/gopls/internal/goxls/typesutil"
)

// GopCollectScopes returns all scopes in an ast path, ordered as innermost scope
// first.
func GopCollectScopes(info *typesutil.Info, path []ast.Node, pos token.Pos) []*types.Scope {
	// scopes[i], where i<len(path), is the possibly nil Scope of path[i].
	var scopes []*types.Scope
	for _, n := range path {
		// Include *FuncType scope if pos is inside the function body.
		switch node := n.(type) {
		case *ast.FuncDecl:
			if node.Body != nil && NodeContains(node.Body, pos) {
				n = node.Type
			}
		case *ast.FuncLit:
			if node.Body != nil && NodeContains(node.Body, pos) {
				n = node.Type
			}
		}
		scopes = append(scopes, info.Scopes[n])
	}
	return scopes
}

// GopQualifier returns a function that appropriately formats a types.PkgName
// appearing in a *ast.File.
func GopQualifier(f *ast.File, pkg *types.Package, info *typesutil.Info) types.Qualifier {
	// Construct mapping of import paths to their defined or implicit names.
	imports := make(map[*types.Package]string)
	for _, imp := range f.Imports {
		if pkgname, ok := GopImportedPkgName(info, imp); ok {
			imports[pkgname.Imported()] = pkgname.Name()
		}
	}
	// Define qualifier to replace full package paths with names of the imports.
	return func(p *types.Package) string {
		if p == pkg {
			return ""
		}
		if name, ok := imports[p]; ok {
			if name == "." {
				return ""
			}
			return name
		}
		return p.Name()
	}
}

// MetadataQualifierForGopFile returns a metadata qualifier that chooses the best
// qualification of an imported package relative to the file f in package with
// metadata m.
func MetadataQualifierForGopFile(s MetadataSource, f *ast.File, m *Metadata) MetadataQualifier {
	// Record local names for import paths.
	localNames := make(map[ImportPath]string) // local names for imports in f
	for _, imp := range f.Imports {
		name, _, impPath, _ := gopImportInfo(s, imp, m)
		localNames[impPath] = name
	}

	// Record a package path -> import path mapping.
	inverseDeps := make(map[PackageID]PackagePath)
	for path, id := range m.DepsByPkgPath {
		inverseDeps[id] = path
	}
	importsByPkgPath := make(map[PackagePath]ImportPath) // best import paths by pkgPath
	for impPath, id := range m.DepsByImpPath {
		if id == "" {
			continue
		}
		pkgPath := inverseDeps[id]
		_, hasPath := importsByPkgPath[pkgPath]
		_, hasImp := localNames[impPath]
		// In rare cases, there may be multiple import paths with the same package
		// path. In such scenarios, prefer an import path that already exists in
		// the file.
		if !hasPath || hasImp {
			importsByPkgPath[pkgPath] = impPath
		}
	}

	return func(pkgName PackageName, impPath ImportPath, pkgPath PackagePath) string {
		// If supplied, translate the package path to an import path in the source
		// package.
		if pkgPath != "" {
			if srcImp := importsByPkgPath[pkgPath]; srcImp != "" {
				impPath = srcImp
			}
			if pkgPath == m.PkgPath {
				return ""
			}
		}
		if localName, ok := localNames[impPath]; ok && impPath != "" {
			return string(localName)
		}
		if pkgName != "" {
			return string(pkgName)
		}
		idx := strings.LastIndexByte(string(impPath), '/')
		return string(impPath[idx+1:])
	}
}

// gopImportInfo collects information about the import specified by imp,
// extracting its file-local name, package name, import path, and package path.
//
// If metadata is missing for the import, the resulting package name and
// package path may be empty, and the file local name may be guessed based on
// the import path.
//
// Note: previous versions of this helper used a PackageID->PackagePath map
// extracted from m, for extracting package path even in the case where
// metadata for a dep was missing. This should not be necessary, as we should
// always have metadata for IDs contained in DepsByPkgPath.
func gopImportInfo(s MetadataSource, imp *ast.ImportSpec, m *Metadata) (string, PackageName, ImportPath, PackagePath) {
	var (
		name    string // local name
		pkgName PackageName
		impPath = UnquoteGopImportPath(imp)
		pkgPath PackagePath
	)

	// If the import has a local name, use it.
	if imp.Name != nil {
		name = imp.Name.Name
	}

	// Try to find metadata for the import. If successful and there is no local
	// name, the package name is the local name.
	if depID := m.DepsByImpPath[impPath]; depID != "" {
		if depm := s.Metadata(depID); depm != nil {
			if name == "" {
				name = string(depm.Name)
			}
			pkgName = depm.Name
			pkgPath = depm.PkgPath
		}
	}

	// If the local name is still unknown, guess it based on the import path.
	if name == "" {
		idx := strings.LastIndexByte(string(impPath), '/')
		name = string(impPath[idx+1:])
	}
	return name, pkgName, impPath, pkgPath
}

// UnquoteImportPath returns the unquoted import path of s,
// or "" if the path is not properly quoted.
func UnquoteGopImportPath(s *ast.ImportSpec) ImportPath {
	path, err := strconv.Unquote(s.Path.Value)
	if err != nil {
		return ""
	}
	return ImportPath(path)
}
*/

// IsGopGenerated gets and reads the file denoted by uri and reports
// whether it contains a "generated file" comment as described at
// https://golang.org/s/generatedcode.
//
// TODO(adonovan): opt: this function does too much.
// Move snapshot.ReadFile into the caller (most of which have already done it).
func IsGopGenerated(ctx context.Context, snapshot Snapshot, uri span.URI) bool {
	return false
}
