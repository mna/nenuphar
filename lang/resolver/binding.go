package resolver

import (
	"fmt"
	"go/ast"
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
)

var scopeNames = [...]string{
	Undefined:   "undefined",
	Local:       "local",
	Cell:        "cell",
	Free:        "free",
	Predeclared: "predeclared",
	Universal:   "universal",
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

	// Index records the index into the enclosing
	// - function's Locals, if Scope==Local
	// - function's FreeVars, if Scope==Free
	// It is zero if Scope is Predeclared, Universal, or Undefined.
	Index int

	// Decl is the statement that declares this binding.
	Decl ast.Stmt
}

type Function struct {
	Definition ast.Node   // can be *Chunk, *ClassStmt, *ClassExpr, *FuncStmt or *FuncExpr
	Locals     []*Binding // this function's local/cell variables, parameters first
	FreeVars   []*Binding // enclosing cells to capture in closure
}
