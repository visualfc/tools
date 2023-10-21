package imports

import (
	"strings"

	"github.com/goplus/gop/ast"
)

// gopExports export Go+ style func, startLower and overload (GopPackage)
func gopExports(scopes []*ast.Scope) (exports []string) {
	var gopPackage bool
	for _, scope := range scopes {
		if obj := scope.Lookup("GopPackage"); obj != nil && obj.Kind == ast.Con {
			gopPackage = true
			break
		}
	}
	for _, scope := range scopes {
		for name, obj := range scope.Objects {
			if ast.IsExported(name) {
				exports = append(exports, name)
				switch obj.Kind {
				case ast.Fun:
					if v, ok := toStartWithLowerCase(name); ok {
						exports = append(exports, v)
					}
					if gopPackage && strings.HasSuffix(name, "__0") {
						name = name[:len(name)-3]
						exports = append(exports, name)
						if v, ok := toStartWithLowerCase(name); ok {
							exports = append(exports, v)
						}
					}
				}
			}
		}
	}
	return exports
}

func toStartWithLowerCase(name string) (string, bool) {
	if c := name[0]; c >= 'A' && c <= 'Z' {
		return string(c+('a'-'A')) + name[1:], true
	}
	return name, false
}
