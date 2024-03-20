package types

import (
	"fmt"

	"github.com/mna/nenuphar/lang/compiler"
)

// A Function is a function defined by a function statement or expression. The
// initialization behavior of a module is also represented by a (top-level)
// Function.
type Function struct {
	Funcode  *compiler.Funcode
	Module   *Module
	Freevars Tuple
}

var (
	_ Value = (*Function)(nil)
)

// A Module is the dynamic counterpart to a compiler.Program, which is the unit
// of compilation. All functions in the same program share a module.
type Module struct {
	Program   *compiler.Program
	Constants []Value
}

func (fn *Function) String() string { return fmt.Sprintf("function(%p %s)", fn, fn.Name()) }
func (fn *Function) Type() string   { return "function" }
func (fn *Function) Name() string {
	nm := fn.Funcode.Name
	if nm == "" {
		if fn.Funcode.Prog.Toplevel == fn.Funcode {
			nm = "toplevel " + fn.Funcode.Prog.Filename
		} else {
			nm = "unknown"
		}
	}
	return nm
}
