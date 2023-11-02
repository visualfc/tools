package typesutil

import (
	"fmt"

	"github.com/goplus/gop/token"
)

type TypeError interface {
	Error() string
}

// GopTypeError for type error of gop files.
// Private fields (`go116*`) in `types.Error` makes it impossible to construct valid
// `types.Error` instances outside of package `go/types`, so we do not use `type.Error`.
type GopTypeError struct {
	Fset *token.FileSet // file set for interpretation of Pos
	Pos  token.Pos      // error position
	Msg  string         // error message
}

// Error returns an error string formatted as follows:
// filename:line:column: message
func (err GopTypeError) Error() string {
	return fmt.Sprintf("%s: %s", err.Fset.Position(err.Pos), err.Msg)
}
