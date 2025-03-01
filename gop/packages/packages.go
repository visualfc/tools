// Copyright 2023 The GoPlus Authors (goplus.org). All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package packages

import (
	"context"
	goast "go/ast"
	"go/types"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"github.com/goplus/gop"
	"github.com/goplus/gop/ast"
	"github.com/goplus/gop/parser"
	"github.com/goplus/gop/scanner"
	"github.com/goplus/gop/token"
	"github.com/goplus/gop/x/typesutil"
	"github.com/goplus/mod/gopmod"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/gop/goputil"
	"golang.org/x/tools/internal/gop/packagesinternal"
	internal "golang.org/x/tools/internal/packagesinternal"
)

type dbgFlags int

const (
	DbgFlagVerbose dbgFlags = 1 << iota
	DbgFlagAll              = DbgFlagVerbose
)

var (
	debugVerbose bool
)

// SetDebug sets debug flags.
func SetDebug(flags dbgFlags) {
	debugVerbose = (flags & DbgFlagVerbose) != 0
}

// An Error describes a problem with a package's metadata, syntax, or types.
type Error = packages.Error

// ErrorKind describes the source of the error, allowing the user to
// differentiate between errors generated by the driver, the parser, or the
// type-checker.
type ErrorKind = packages.ErrorKind

const (
	UnknownError = packages.UnknownError
	ListError    = packages.ListError
	ParseError   = packages.ParseError
	TypeError    = packages.TypeError
)

// A LoadMode controls the amount of detail to return when loading.
// The bits below can be combined to specify which fields should be
// filled in the result packages.
// The zero value is a special case, equivalent to combining
// the NeedName, NeedFiles, and NeedCompiledGoFiles bits.
// ID and Errors (if present) will always be filled.
// Load may return more information than requested.
type LoadMode = packages.LoadMode

const (
	// NeedName adds Name and PkgPath.
	NeedName = packages.NeedName

	// NeedFiles adds GoFiles, GopFiles and OtherFiles.
	NeedFiles = packages.NeedFiles

	// NeedCompiledGoFiles adds CompiledGoFiles/CompiledGopFiles.
	NeedCompiledGoFiles = packages.NeedCompiledGoFiles

	// NeedCompiledGopFiles adds CompiledGoFiles/CompiledGopFiles.
	NeedCompiledGopFiles = packages.NeedCompiledGoFiles

	// NeedImports adds Imports. If NeedDeps is not set, the Imports field will contain
	// "placeholder" Packages with only the ID set.
	NeedImports = packages.NeedImports

	// NeedDeps adds the fields requested by the LoadMode in the packages in Imports.
	NeedDeps = packages.NeedDeps

	// NeedExportFile adds ExportFile.
	NeedExportFile = packages.NeedExportFile

	// NeedTypes adds Types, Fset, and IllTyped.
	NeedTypes = packages.NeedTypes

	// NeedSyntax adds Syntax.
	NeedSyntax = packages.NeedSyntax

	// NeedTypesInfo adds TypesInfo.
	NeedTypesInfo = packages.NeedTypesInfo

	// NeedTypesSizes adds TypesSizes.
	NeedTypesSizes = packages.NeedTypesSizes

	// NeedModule adds Module.
	NeedModule = packages.NeedModule

	// NeedEmbedFiles adds EmbedFiles.
	NeedEmbedFiles = packages.NeedEmbedFiles

	// NeedEmbedPatterns adds EmbedPatterns.
	NeedEmbedPatterns = packages.NeedEmbedPatterns

	// NeedNongen adds CompiledNongenGoFiles, NongenSyntax.
	NeedNongen = LoadMode(1 << 30)
)

const (
	// Deprecated: LoadFiles exists for historical compatibility
	// and should not be used. Please directly specify the needed fields using the Need values.
	LoadFiles = packages.LoadFiles

	// Deprecated: LoadImports exists for historical compatibility
	// and should not be used. Please directly specify the needed fields using the Need values.
	LoadImports = packages.LoadImports

	// Deprecated: LoadTypes exists for historical compatibility
	// and should not be used. Please directly specify the needed fields using the Need values.
	LoadTypes = packages.LoadTypes

	// Deprecated: LoadSyntax exists for historical compatibility
	// and should not be used. Please directly specify the needed fields using the Need values.
	LoadSyntax = packages.LoadSyntax

	// Deprecated: LoadAllSyntax exists for historical compatibility
	// and should not be used. Please directly specify the needed fields using the Need values.
	LoadAllSyntax = packages.LoadAllSyntax

	// Deprecated: NeedExportsFile is a historical misspelling of NeedExportFile.
	NeedExportsFile = packages.NeedExportsFile
)

// A Config specifies details about how packages should be loaded.
// The zero value is a valid configuration.
// Calls to Load do not modify this struct.
type Config = packages.Config

// A Package describes a loaded Go+ package.
type Package struct {
	packages.Package

	// GopFiles lists the absolute file paths of the package's Go source files.
	// It may include files that should not be compiled, for example because
	// they contain non-matching build tags, are documentary pseudo-files such as
	// unsafe/unsafe.go or builtin/builtin.go, or are subject to cgo preprocessing.
	GopFiles []string

	// CompiledGopFiles lists the absolute file paths of the package's source
	// files that are suitable for type checking.
	// This may differ from GoFiles if files are processed before compilation.
	CompiledGopFiles []string

	// Imports maps import paths appearing in the package's Go/Go+ source files
	// to corresponding loaded Packages.
	Imports map[string]*Package

	// CompiledNongenGoFiles lists the absolute file paths of the package's source
	// files that are suitable for type checking.
	// This may differ from GoFiles if files are processed before compilation.
	CompiledNongenGoFiles []string

	// NongenSyntax is the package's syntax trees, for the files listed in CompiledNongenGoFiles.
	//
	// The NeedSyntax LoadMode bit populates this field for packages matching the patterns.
	// If NeedDeps and NeedImports are also set, this field will also be populated
	// for dependencies.
	//
	// NongenSyntax is kept in the same order as CompiledNongenGoFiles, with the caveat that nils are
	// removed.  If parsing returned nil, nongenSyntax may be shorter than CompiledNongenGoFiles.
	NongenSyntax []*goast.File

	// GopSyntax is the package's syntax trees, for the files listed in CompiledGopFiles.
	//
	// The NeedSyntax LoadMode bit populates this field for packages matching the patterns.
	// If NeedDeps and NeedImports are also set, this field will also be populated
	// for dependencies.
	//
	// GopSyntax is kept in the same order as CompiledGopFiles, with the caveat that nils are
	// removed. If parsing returned nil, GopSyntax may be shorter than CompiledGopFiles.
	GopSyntax []*ast.File

	// GopTypesInfo provides type information about the package's syntax trees.
	// It is set only when GopSyntax is set.
	GopTypesInfo *typesutil.Info
}

// A GopConfig specifies details about how Go+ packages should be loaded.
type GopConfig struct {
	// ParseFile is called to read and parse each file
	// when preparing a package's type-checked syntax tree.
	// It must be safe to call ParseFile simultaneously from multiple goroutines.
	// If ParseFile is nil, the loader will uses parser.ParseFile.
	//
	// ParseFile should parse the source from src and use filename only for
	// recording position information.
	//
	// An application may supply a custom implementation of ParseFile
	// to change the effective file contents or the behavior of the parser,
	// or to modify the syntax tree. For example, selectively eliminating
	// unwanted function bodies can significantly accelerate type checking.
	ParseFile func(fset *token.FileSet, filename string, src any, cfg parser.Config) (*ast.File, error)

	// Context is an opaque packages.Load context.
	// Contexts are safe for concurrent use.
	Context *Context
}

// Load loads and returns the Go/Go+ packages named by the given patterns.
//
// Config specifies loading options;
// nil behaves the same as an empty Config.
//
// Load returns an error if any of the patterns was invalid
// as defined by the underlying build system.
// It may return an empty list of packages without an error,
// for instance for an empty expansion of a valid wildcard.
// Errors associated with a particular package are recorded in the
// corresponding Package's Errors list, and do not cause Load to
// return an error. Clients may need to handle such errors before
// proceeding with further analysis. The PrintErrors function is
// provided for convenient display of all errors.
func Load(cfg *Config, patterns ...string) ([]*Package, error) {
	return LoadEx(nil, cfg, patterns...)
}

// LoadEx loads and returns the Go/Go+ packages named by the given patterns.
//
// Config specifies loading options;
// nil behaves the same as an empty Config.
//
// LoadEx returns an error if any of the patterns was invalid
// as defined by the underlying build system.
// It may return an empty list of packages without an error,
// for instance for an empty expansion of a valid wildcard.
// Errors associated with a particular package are recorded in the
// corresponding Package's Errors list, and do not cause Load to
// return an error. Clients may need to handle such errors before
// proceeding with further analysis. The PrintErrors function is
// provided for convenient display of all errors.
func LoadEx(gop *GopConfig, cfg *Config, patterns ...string) ([]*Package, error) {
	patterns, _ = GenGo(patterns...)

	var conf Config
	if cfg != nil {
		conf = *cfg
	}
	if conf.Fset == nil {
		conf.Fset = token.NewFileSet()
	}
	if conf.Mode == 0 {
		conf.Mode = NeedName | NeedFiles | NeedCompiledGoFiles
	}
	pkgs, err := packages.Load(&conf, patterns...)
	if err != nil {
		return nil, err
	}

	pkgMap := make(map[*packages.Package]*Package)
	ret := make([]*Package, len(pkgs))

	var ld *loader
	if conf.Mode&(NeedSyntax|NeedTypes|NeedTypesInfo) != 0 {
		if conf.Context == nil {
			conf.Context = context.Background()
		}
		if gop == nil {
			gop = new(GopConfig)
		}
		ctx := gop.Context
		if ctx == nil {
			ctx = Default
		}
		parse := gop.ParseFile
		if parse == nil {
			parse = parser.ParseEntry
		}
		ld = &loader{conf.Fset, ctx, parse, conf.Overlay, conf.Context}
	}

	for i, pkg := range pkgs {
		ret[i] = pkgOf(pkgMap, pkg, ld, conf.Mode)
	}
	return ret, nil
}

func importPkgs(pkgMap map[*packages.Package]*Package, pkgs map[string]*packages.Package, ld *loader, mode LoadMode) map[string]*Package {
	if len(pkgs) == 0 {
		return nil
	}
	ret := make(map[string]*Package, len(pkgs))
	for path, pkg := range pkgs {
		ret[path] = pkgOf(pkgMap, pkg, ld, mode)
	}
	return ret
}

func initNongen(ret *Package, i int) {
	n := len(ret.CompiledGoFiles)
	ret.CompiledNongenGoFiles = make([]string, i, n)
	copy(ret.CompiledNongenGoFiles, ret.CompiledGoFiles)
	for i++; i < n; i++ {
		file := ret.CompiledGoFiles[i]
		if isAutogen(filepath.Base(file)) {
			continue
		}
		ret.CompiledNongenGoFiles = append(ret.CompiledNongenGoFiles, file)
	}
	ret.NongenSyntax = make([]*goast.File, 0, n)
	fset := ret.Fset
	for _, f := range ret.Syntax {
		file := fset.File(f.Pos()).Name()
		if isAutogen(filepath.Base(file)) {
			continue
		}
		ret.NongenSyntax = append(ret.NongenSyntax, f)
	}
}

func pkgOf(pkgMap map[*packages.Package]*Package, pkg *packages.Package, ld *loader, mode LoadMode) *Package {
	if ret, ok := pkgMap[pkg]; ok {
		return ret
	}
	ret := &Package{Package: *pkg, Imports: importPkgs(pkgMap, pkg.Imports, ld, mode)}
	needNongen := (mode & NeedNongen) != 0
	if needNongen {
		ret.CompiledNongenGoFiles = pkg.CompiledGoFiles
		ret.NongenSyntax = pkg.Syntax
	}
	for i, file := range pkg.CompiledGoFiles {
		dir, fname := filepath.Split(file)
		if isAutogen(fname) { // has Go+ files
			addGopFiles(ret, ld, dir, mode, isGoTestFile(fname) || hasGoTestFile(pkg.CompiledGoFiles[i+1:]))
			if needNongen {
				initNongen(ret, i)
			}
			break
		}
	}
	pkgMap[pkg] = ret
	return ret
}

func hasGoTestFile(goFiles []string) bool {
	for _, file := range goFiles {
		fname := filepath.Base(file)
		if isGoTestFile(fname) {
			return true
		}
	}
	return false
}

func isAutogen(fname string) bool {
	return strings.HasPrefix(fname, "gop_autogen")
}

func isGoTestFile(fname string) bool {
	return strings.HasSuffix(fname, "_test.go")
}

func autogenFiles(ret *Package, test bool) []*goast.File {
	files := make([]*goast.File, 0, 2)
	fset := ret.Fset
	for _, f := range ret.Syntax {
		file := fset.File(f.Pos()).Name()
		fname := filepath.Base(file)
		if isAutogen(fname) {
			if !isGoTestFile(fname) || test {
				files = append(files, f)
			}
		}
	}
	return files
}

func addGopFiles(ret *Package, ld *loader, dir string, mode LoadMode, test bool) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	fsetTemp := token.NewFileSet()
	pkgName := ret.Name
	var mod *gopmod.Module
	var once sync.Once
	for _, e := range entries {
		fname := e.Name()
		if strings.HasPrefix(fname, "_") {
			continue
		}
		fext := path.Ext(fname)
		if goputil.FileKind(fext) == goputil.FileUnknown {
			continue
		}
		if !test {
			if strings.HasSuffix(fname[:len(fname)-len(fext)], "_test") {
				continue
			}
			// check gox class test
			if strings.HasSuffix(fname, "test.gox") {
				once.Do(func() {
					mod, _ = gop.LoadMod(dir)
				})
				if mod != nil {
					if _, ok := mod.ClassKind(fname); ok {
						continue
					}
				}
			}
		}
		file := dir + fname
		f, err := parser.ParseFile(fsetTemp, file, nil, parser.PackageClauseOnly)
		if err == nil && pkgName == f.Name.Name {
			ret.GopFiles = append(ret.GopFiles, file)
			ret.CompiledGopFiles = append(ret.CompiledGopFiles, file)
			// goxls: todo - condition
		}
	}
	if ld != nil && len(ret.CompiledGopFiles) > 0 {
		ctx := ld.Context
		mod := ctx.LoadMod(ret.Module)
		ret.GopSyntax = ld.parseFiles(ret, mod, ret.CompiledGopFiles)
		if mode&(NeedTypes|NeedTypesInfo) != 0 {
			ret.GopTypesInfo = &typesutil.Info{
				Types:      make(map[ast.Expr]types.TypeAndValue),
				Defs:       make(map[*ast.Ident]types.Object),
				Uses:       make(map[*ast.Ident]types.Object),
				Implicits:  make(map[ast.Node]types.Object),
				Scopes:     make(map[ast.Node]*types.Scope),
				Selections: make(map[*ast.SelectorExpr]*types.Selection),
				Instances:  make(map[*ast.Ident]types.Instance),
				Overloads:  make(map[*ast.Ident][]types.Object),
			}
			cfg := &types.Config{
				Context: ctx.Types,
				Error: func(err error) {
					appendError(ret, err)
				},
			}
			opts := &typesutil.Config{
				Types: ret.Types,
				Fset:  ld.Fset,
				Mod:   mod,
			}

			scope := ret.Types.Scope()
			objMap := typesutil.DeleteObjects(scope, autogenFiles(ret, test))
			c := typesutil.NewChecker(cfg, opts, nil, ret.GopTypesInfo)
			err = c.Files(nil, ret.GopSyntax)
			typesutil.CorrectTypesInfo(scope, objMap, ret.TypesInfo.Uses)
			if err != nil && debugVerbose {
				log.Println("typesutil.Check:", err)
			}
		}
	}
}

type loader struct {
	Fset      *token.FileSet
	Context   *Context
	ParseFile func(fset *token.FileSet, filename string, src any, cfg parser.Config) (*ast.File, error)

	// Overlay provides a mapping of absolute file paths to file contents.
	// If the file with the given path already exists, the parser will use the
	// alternative file contents provided by the map.
	//
	// Overlays provide incomplete support for when a given file doesn't
	// already exist on disk. See the package doc above for more details.
	Overlay map[string][]byte

	ctx context.Context
}

// parseFiles reads and parses the Go+ source files and returns the ASTs
// of the ones that could be at least partially parsed, along with a
// list of I/O and parse errors encountered.
//
// Because files are scanned in parallel, the token.Pos
// positions of the resulting ast.Files are not ordered.
func (ld *loader) parseFiles(ret *Package, mod *gopmod.Module, filenames []string) []*ast.File {
	var wg sync.WaitGroup
	n := len(filenames)
	ctx := ld.ctx
	parsed := make([]*ast.File, n)
	errors := make([]error, n)
	for i, file := range filenames {
		if ctx.Err() != nil {
			parsed[i] = nil
			errors[i] = ctx.Err()
			continue
		}
		wg.Add(1)
		go func(i int, filename string) {
			defer wg.Done()
			parsed[i], errors[i] = ld.parseFile(filename, mod)
		}(i, file)
	}
	wg.Wait()

	for _, err := range errors {
		if err != nil {
			appendError(ret, err)
		}
	}

	// Eliminate nils, preserving order.
	var o int
	for _, f := range parsed {
		if f != nil {
			parsed[o] = f
			o++
		}
	}
	return parsed[:o]
}

// We use a counting semaphore to limit
// the number of parallel I/O calls per process.
var ioLimit = make(chan bool, 20)

func (ld *loader) parseFile(filename string, mod *gopmod.Module) (f *ast.File, err error) {
	var src []byte
	for f, contents := range ld.Overlay {
		if sameFile(f, filename) {
			src = contents
		}
	}
	if src == nil {
		ioLimit <- true // wait
		src, err = os.ReadFile(filename)
		<-ioLimit // signal
	}
	if err != nil {
		return
	}
	if debugVerbose {
		log.Println("==> ld.parseFile:", filename, "fset:", ld.Fset != nil, "ld.ParseFile:", ld.ParseFile != nil)
	}
	return ld.ParseFile(ld.Fset, filename, src, parser.Config{
		Mode:      parser.AllErrors | parser.ParseComments,
		ClassKind: mod.ClassKind,
	})
}

// sameFile returns true if x and y have the same basename and denote
// the same file.
func sameFile(x, y string) bool {
	if x == y {
		// It could be the case that y doesn't exist.
		// For instance, it may be an overlay file that
		// hasn't been written to disk. To handle that case
		// let x == y through. (We added the exact absolute path
		// string to the CompiledGoFiles list, so the unwritten
		// overlay case implies x==y.)
		return true
	}
	if strings.EqualFold(filepath.Base(x), filepath.Base(y)) { // (optimisation)
		if xi, err := os.Stat(x); err == nil {
			if yi, err := os.Stat(y); err == nil {
				return os.SameFile(xi, yi)
			}
		}
	}
	return false
}

func appendError(ret *Package, err error) {
	switch err := err.(type) {
	case Error:
		// from driver
		ret.Errors = append(ret.Errors, err)

	case *os.PathError:
		// from parser
		ret.Errors = append(ret.Errors, Error{
			Pos:  err.Path + ":1",
			Msg:  err.Err.Error(),
			Kind: ParseError,
		})

	case scanner.ErrorList:
		// from parser
		for _, err := range err {
			ret.Errors = append(ret.Errors, Error{
				Pos:  err.Pos.String(),
				Msg:  err.Msg,
				Kind: ParseError,
			})
		}

	case types.Error:
		// from type checker
		ret.TypeErrors = append(ret.TypeErrors, err)
		ret.Errors = append(ret.Errors, Error{
			Pos:  err.Fset.Position(err.Pos).String(),
			Msg:  err.Msg,
			Kind: TypeError,
		})

	default:
		// unexpected impoverished error from parser?
		ret.Errors = append(ret.Errors, Error{
			Pos:  "-",
			Msg:  err.Error(),
			Kind: UnknownError,
		})

		// If you see this error message, please file a bug.
		log.Printf("internal error: error %q (%T) without position", err, err)
	}
}

func init() {
	packagesinternal.GetForTest = func(p interface{}) string {
		return internal.GetForTest(&p.(*Package).Package)
	}
	packagesinternal.GetDepsErrors = func(p interface{}) []*packagesinternal.PackageError {
		return internal.GetDepsErrors(&p.(*Package).Package)
	}
	packagesinternal.GetGoCmdRunner = internal.GetGoCmdRunner
	packagesinternal.SetGoCmdRunner = internal.SetGoCmdRunner
	packagesinternal.SetModFile = internal.SetModFile
	packagesinternal.SetModFlag = internal.SetModFlag
	packagesinternal.TypecheckCgo = internal.TypecheckCgo
	packagesinternal.DepsErrors = internal.DepsErrors
	packagesinternal.ForTest = internal.ForTest
}
