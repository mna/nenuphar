// Package resolver implements the resolver that takes a parsed abstract syntax
// tree and resolves the identifiers to bindings.
//
// # Scopes
//
// Bindings are either "undefined" (which generates an error), "local" to a
// function (which may be the top-level), a "free" binding (a reference to a
// binding declared in an enclosing function, i.e. a closure), a "label" (which
// has special scoping rules, see below), "predeclared" (from a list of
// bindings provided to the environment) or from the "universe" (bindings that
// are built into the language). There is no concept of global variables.
//
// When a local binding is used as a free binding, it becomes a "cell" (a
// function's local that is shared with at least one nested function).
//
// # Labels
//
// A label is visible in the entire block where it is defined, except
// inside nested functions and defer/catch blocks (it is strictly
// local). A goto may jump to any visible label as long as it does not
// enter into the scope of a local binding. A label cannot be declared
// where a label with the same name is visible, even if this other label
// has been declared in an enclosing block.
//
// A break or continue statement can only reference a label associated with a
// loop, and respectively breaks out or starts next iteration of the referenced
// loop. A label is associated with a loop if it immediately precedes the loop
// statement (ignoring whitespace and comments).
//
// # Bindings
//
// The following statements define new bindings:
//   - BindIfStmt: e.g. "if let x = 1 then .. end". The scope of the
//     bindings are limited to the "true" block of the "if" statement.
//   - BindGuardStmt: e.g. "guard let x = 1 else .. end". The scope of
//     the bindings the enclosing block of the guard statement (from that
//     point on).
//   - ThreePartForStmt: e.g. "for let x = 1; .. end". The scope of the
//     bindings are limited to the body of the "for" loop.
//   - ForInStmt: e.g. "for x, y, x in .. end". The scope of the
//     bindings are limited to the body of the "for" loop. New bindings
//     are always defined for this syntax (implicit "let").
//   - FuncStmt: e.g. "fn foo() .. end". The scope of the name of the
//     function is the enclosing block (from this point on). The scope of
//     the parameters of the function are limited to the body of the
//     function. New bindings are always defined for function parameters.
//   - LabelStmt: e.g. "::foo::". Defines a new "label" binding, visible
//     anywhere in its enclosing block as per the label scope rules.
//   - DeclStmt: e.g. "const x = 1". The scope of the bindings are the
//     enclosing block (from this point on).
//   - ClassStmt: e.g. "class Foo .. end". The scope of the name of the
//     class is the enclosing block (from this point on).
//   - MethodDef: e.g. "fn Bar() .. end" inside a class. TBD.
//   - FieldDef: e.g. "let x = 1" inside a class. TBD.
package resolver

import (
	"context"
	"fmt"
	"go/scanner"
	"go/token"

	"github.com/mna/nenuphar/lang/ast"
)

// ResolveFiles takes the file set and corresponding list of chunks from a
// successful parse result and resolves the bindings used in the source code.
// On success, the AST is enriched with binding resolution information and is
// ready to be compiled to bytecode for virtual machine execution.
//
// An AST that resulted in errors in the parse phase should never be passed to
// the resolver, the behavior is undefined.
//
// The returned error, if non-nil, is guaranteed to be a scanner.ErrorList.
func ResolveFiles(ctx context.Context, fset *token.FileSet, chunks []*ast.Chunk,
	isPredeclared, isUniversal func(name string) bool) error {
	if len(chunks) == 0 {
		return nil
	}

	var r resolver
	r.isPredeclared = isPredeclared
	r.isUniversal = isUniversal
	for _, ch := range chunks {
		start, _ := ch.Span()
		r.init(fset.File(start))
		// TODO: r.block(ch.Block)
	}
	r.errors.Sort()
	return r.errors.Err()
}

type resolver struct {
	file *token.File

	// env is the current local environment, a linked list of blocks, with the
	// current innermost block first and the tail of the list the file
	// (top-level) block.
	env *block

	// globals saves the bindings of predeclared and universal names when they
	// are first referenced.
	globals map[string]*Binding

	// predicates to check if an unresolved name is predeclared or universal.
	isPredeclared, isUniversal func(name string) bool

	// number of enclosing for loops
	loops int

	errors scanner.ErrorList
}

func (r *resolver) init(file *token.File) {
	r.file = file
	r.env = nil
	r.globals = make(map[string]*Binding)
	r.loops = 0
}

func (r *resolver) push(b *block) {
	r.env.children = append(r.env.children, b)
	b.parent = r.env
	r.env = b
}

func (r *resolver) pop() {
	r.env = r.env.parent
}

func (r *resolver) errorf(p token.Pos, format string, args ...interface{}) {
	r.errors.Add(r.file.Position(p), fmt.Sprintf(format, args...))
}

type block struct {
	parent *block // nil for file block
	fn     *Function

	// indicates if this is the top-level block of a defer or a catch, which
	// cannot "see" labels in the parent blocks.
	isDeferCatch bool

	// bindings maps a name to its binding. A local binding has an index
	// into its innermost enclosing container's locals array. A free
	// binding has an index into its innermost enclosing function's
	// freevars array.
	bindings map[string]*Binding

	// children records the child blocks of the current one.
	children []*block
}
