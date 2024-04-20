package resolver

import (
	"fmt"

	"github.com/mna/nenuphar/lang/ast"
)

// The Scope of Binding indicates what kind of scope it has.
type Scope uint8

const (
	Undefined   Scope = iota // name is not defined
	Local                    // name is local to its function
	Cell                     // name is function-local but shared with a nested function
	Free                     // name is cell of some enclosing function
	Predeclared              // name is predeclared for this module (provided to its environment)
	Universal                // name is universal (a language built-in)
	Label                    // name is a label
)

var scopeNames = [...]string{
	Undefined:   "undefined",
	Local:       "local",
	Cell:        "cell",
	Free:        "free",
	Predeclared: "predeclared",
	Universal:   "universal",
	Label:       "label",
}

func (s Scope) String() string {
	if int(s) >= len(scopeNames) {
		return fmt.Sprintf("<invalid Scope %d>", s)
	}
	return scopeNames[s]
}

// A Binding contains resolver information about an identifier. The resolver
// creates a binding for each declaration and it ties together all identifiers
// that denote the same variable.
type Binding struct {
	Scope Scope

	// Const is true if the binding is a constant.
	Const bool

	// Index records the index into the enclosing
	// - function's Locals, if Scope==Local
	// - function's FreeVars, if Scope==Free
	// - function's Labels, if Scope==Label
	// It is zero if Scope is Predeclared, Universal, or Undefined.
	Index int

	// Decl is the declaration node of this binding, or first reference for a
	// global binding (predeclared or universal).
	Decl *ast.IdentExpr
}

type Function struct {
	Definition ast.Node   // can be *Chunk, *ClassStmt, *ClassExpr, *FuncStmt or *FuncExpr
	HasVarArg  bool       // for function, if last parameter is vararg
	Locals     []*Binding // this function's local/cell variables, parameters first
	FreeVars   []*Binding // enclosing cells to capture in closure
	Labels     []*Binding // the labels defined in this function

	// number of enclosing for loops
	loops int
	// number of enclosing catch blocks
	catches int
	// number of enclosing defer blocks
	defers int
}

// IsClass indicates if the function is a class, which has different scoping
// rules (TODO: ?).
func (f *Function) IsClass() bool {
	switch f.Definition.(type) {
	case *ast.ClassStmt:
		return true
	case *ast.ClassExpr:
		return true
	default:
		return false
	}
}
