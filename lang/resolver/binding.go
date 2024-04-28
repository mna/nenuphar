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
	LoopLabel                // name is a label associated with a loop
)

var scopeNames = [...]string{
	Undefined:   "undefined",
	Local:       "local",
	Cell:        "cell",
	Free:        "free",
	Predeclared: "predeclared",
	Universal:   "universal",
	Label:       "label",
	LoopLabel:   "loop label",
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

	// BlockName uniquely identifies the block where this binding is defined.
	// Only set if the resolver is done with tne NameBlocks option.
	BlockName string
}

func (b *Binding) FormatFor(id *ast.IdentExpr) string {
	var s string
	if b.Decl == id {
		s = "++ "
	} else {
		s = "-> "
	}
	switch b.Scope {
	case Undefined:
		s += "undef"
	case Local:
		if b.Const {
			s += "const"
		} else {
			s += "let"
		}
	case Free:
		if b.Const {
			s += "free const"
		} else {
			s += "free let"
		}
	case Cell:
		if b.Const {
			s += "cell const"
		} else {
			s += "cell let"
		}
	case Predeclared:
		s += "pre"
	case Universal:
		s += "univ"
	case Label:
		s += "label"
	case LoopLabel:
		s += "loop label"
	}
	if b.BlockName != "" {
		s += " (" + b.BlockName + ")"
	}
	return s
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

// IsClass indicates if the function is a class.
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

type block struct {
	parent *block // nil for file block
	fn     *Function
	name   string // set only if NameBlocks option is set

	// indicates if this is the top-level block of a defer or a catch, which
	// cannot "see" labels in the parent blocks.
	isDeferCatch bool

	// bindings and lbindings maps a name to its binding (for variables and
	// labels, respectively). A local binding has an index into its innermost
	// enclosing function's locals array. A free binding has an index into its
	// innermost enclosing function's freevars array.
	bindings  map[string]*Binding
	lbindings map[string]*Binding

	// children records the child blocks of the current one.
	children []*block
}
