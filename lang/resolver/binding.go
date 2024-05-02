package resolver

import (
	"fmt"
	"strings"

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

	// Const is true if the binding is a constant. In addition to explicit const
	// variables, function and class names are constant, and so are predeclared
	// and universal identifiers.
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
	if b.Decl == id && b.Scope != Universal && b.Scope != Predeclared {
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

	// stack of enclosing loop, catch and defer blocks. For loops, if there is a
	// matching label the string is "loop:<labelname>", otherwise the blocks are
	// identified by "loop", "defer" and "catch".
	lcdStack []string

	// pendingLoopLabel is set to the name of a label associated with a loop in
	// the short interval where the label has been processed but the loop is
	// upcoming. Once the loop is entered, it gets cleared and inserted in the
	// lcdStack.
	pendingLoopLabel string
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

	// pendingLabels are labels that have been used but have yet to be defined.
	// On function or defer/catch exit, all pending labels must have been
	// defined, otherwise it is an undefined label error at the point of use.
	pendingLabels map[string]*Binding

	// children records the child blocks of the current one.
	children []*block
}

// isInDefer returns true if the block is inside a defer (possibly nested in
// catch or loop blocks).
func (b *block) isInDefer() bool {
	for i := len(b.fn.lcdStack) - 1; i >= 0; i-- {
		if b.fn.lcdStack[i] == "defer" {
			return true
		}
	}
	return false
}

// isInCatch returns true if the block is inside a catch (possibly nested in
// defer or loop blocks).
func (b *block) isInCatch() bool {
	for i := len(b.fn.lcdStack) - 1; i >= 0; i-- {
		if b.fn.lcdStack[i] == "catch" {
			return true
		}
	}
	return false
}

// isDirectlyInLoop returns true if the block is directly inside a loop,
// without a defer or catch block in between.
func (b *block) isDirectlyInLoop() bool {
	return len(b.fn.lcdStack) > 0 && strings.HasPrefix(b.fn.lcdStack[len(b.fn.lcdStack)-1], "loop")
}

// isValidLoopLabel checks in the current and enclosing loops if any are
// associated with a label with the specified name and returns true if this is
// the case. A defer or catch block acts as a barrier and labels defined higher
// up the stack are not visible nor valid.
func (b *block) isValidLoopLabel(name string) bool {
	for i := len(b.fn.lcdStack) - 1; i >= 0; i-- {
		lcd, lbl, _ := strings.Cut(b.fn.lcdStack[i], ":")
		if lcd != "loop" {
			break
		}
		if lbl == name {
			return true
		}
	}
	return false
}
