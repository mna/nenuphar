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

	"github.com/mna/nenuphar/lang/ast"
	"github.com/mna/nenuphar/lang/scanner"
	"github.com/mna/nenuphar/lang/token"
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
		r.block(ch.Block, ch)
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

	errors scanner.ErrorList
}

func (r *resolver) init(file *token.File) {
	r.file = file
	r.env = nil
	r.globals = make(map[string]*Binding)
}

func (r *resolver) push(b *block) {
	if r.env != nil {
		r.env.children = append(r.env.children, b)
		if b.fn == nil {
			// in same function as before
			b.fn = r.env.fn
		}
	}
	b.parent = r.env
	r.env = b
}

func (r *resolver) pop() {
	r.env = r.env.parent
}

func (r *resolver) errorf(p token.Pos, format string, args ...interface{}) {
	r.errors.Add(r.file.Position(p), fmt.Sprintf(format, args...))
}

func (r *resolver) block(b *ast.Block, from ast.Node) {
	var blk block
	switch v := from.(type) {
	case *ast.Chunk:
		blk.fn = &Function{Definition: v}
	}

	r.push(&blk)
	for _, s := range b.Stmts {
		r.stmt(s)
	}
	r.pop()
}

func (r *resolver) stmt(stmt ast.Stmt) {
	switch stmt := stmt.(type) {
	case *ast.AssignStmt:
		// resolve the rhs first
		for _, e := range stmt.Right {
			r.expr(e)
		}

		for _, e := range stmt.Left {
			if stmt.DeclType != token.ILLEGAL {
				// this is a declaration, create a new binding
				r.bind(e.(*ast.IdentExpr), stmt.DeclType == token.CONST)
			} else {
				r.expr(e)
			}
		}

	case *ast.ClassStmt:

	case *ast.ExprStmt:
		r.expr(stmt.Expr)

	case *ast.ForInStmt:

	case *ast.ForLoopStmt:
		//if !r.options.TopLevelControl && r.container().function == nil {
		//	r.errorf(stmt.For, "for loop not within a function")
		//}
		//r.expr(stmt.X)
		//const isAugmented = false
		//r.assign(stmt.Vars, isAugmented)
		//r.loops++
		//r.stmts(stmt.Body)
		//r.loops--

	case *ast.FuncStmt:
		//r.bind(stmt.Name)
		//fn := &Function{
		//	Name:   stmt.Name.Name,
		//	Pos:    stmt.Def,
		//	Params: stmt.Params,
		//	Body:   stmt.Body,
		//}
		//stmt.Function = fn
		//r.function(fn, stmt.Def)

	case *ast.IfGuardStmt:
		//if !r.options.TopLevelControl && r.container().function == nil {
		//	r.errorf(stmt.If, "if statement not within a function")
		//}
		//r.expr(stmt.Cond)
		//r.ifstmts++
		//r.stmts(stmt.True)
		//r.stmts(stmt.False)
		//r.ifstmts--

	case *ast.LabelStmt:
		loop := stmt.Next != nil && stmt.Next.IsLoop()
		r.bindLabel(stmt.Name, loop)

	case *ast.ReturnLikeStmt:
		// break, continue and goto must refer to a valid label
		if stmt.Type == token.BREAK || stmt.Type == token.CONTINUE || stmt.Type == token.GOTO {
			if stmt.Expr != nil {
				requireLoopLabel := stmt.Type != token.GOTO
				r.useLabel(stmt.Expr.(*ast.IdentExpr), requireLoopLabel)
			}
			break
		}

		// return or throw is a standard expression
		if stmt.Type == token.RETURN {
			if r.env.fn.defers > 0 {
				// TODO: return cannot be in a defer
			}
		} else if stmt.Type == token.THROW {
			if stmt.Expr == nil && r.env.fn.catches == 0 {
				// TODO: cannot throw without expression outside of a catch
			}
		}

		if stmt.Expr != nil {
			r.expr(stmt.Expr)
		}

	case *ast.SimpleBlockStmt:

	default:
		panic(fmt.Sprintf("unexpected stmt %T", stmt))
	}
}

func (r *resolver) expr(expr ast.Expr) {
	switch expr := expr.(type) {
	case *ast.ArrayLikeExpr:
		for _, e := range expr.Items {
			r.expr(e)
		}

	case *ast.BinOpExpr:
		r.expr(expr.Left)
		r.expr(expr.Right)

	case *ast.CallExpr:
		r.expr(expr.Fn)
		for _, e := range expr.Args {
			r.expr(e)
		}
		// TODO: fail gracefully when > max args?

	case *ast.ClassExpr:

	case *ast.DotExpr:
		// ignore right, can be anything (runtime lookup)
		r.expr(expr.Left)

	case *ast.FuncExpr:

	case *ast.IdentExpr:
		r.use(expr)

	case *ast.IndexExpr:
		r.expr(expr.Prefix)
		r.expr(expr.Index)

	case *ast.LiteralExpr:
		// nothing to do

	case *ast.MapExpr:
		for _, it := range expr.Items {
			r.expr(it.Key)
			r.expr(it.Value)
		}

	case *ast.ParenExpr:
		r.expr(expr.Expr)

	case *ast.UnaryOpExpr:
		r.expr(expr.Right)

	default:
		panic(fmt.Sprintf("unexpected expr %T", expr))
	}
}

func (r *resolver) bind(ident *ast.IdentExpr, isConst bool) {
	if _, ok := r.env.bindings[ident.Lit]; ok {
		// rule: can only shadow in a child block
		r.errorf(ident.Start, "already declared in this block: %s", ident.Lit)
		return
	}

	bdg := &Binding{Scope: Local, Const: isConst, Decl: ident}
	ix := len(r.env.fn.Locals)
	bdg.Index = ix
	r.env.fn.Locals = append(r.env.fn.Locals, bdg)
	r.env.bindings[ident.Lit] = bdg

	ident.Binding = bdg
}

func (r *resolver) bindLabel(ident *ast.IdentExpr, loop bool) {
	// rule: labels cannot be shadowed, and a label cannot shadow a variable.
	//	if _, ok := r.env.bindings[ident.Lit]; ok {
	//		r.errorf(ident.Start, "already declared in this block: %s", ident.Lit)
	//		return
	//	}
}

func (r *resolver) use(ident *ast.IdentExpr) {
	startFn := r.env.fn
	for env := r.env; env != nil; env = env.parent {
		if bdg := env.bindings[ident.Lit]; bdg != nil && bdg.Scope != Label && bdg.Scope != LoopLabel {
			if env.fn != startFn {
				// Found in a parent block which belongs to enclosing function. Add the
				// parent's binding to the function's freevars, and add a new 'free'
				// binding to the inner function's block, and turn the parent's local
				// into cell.
				if bdg.Scope == Local {
					bdg.Scope = Cell
				}
				ix := len(r.env.fn.FreeVars)
				r.env.fn.FreeVars = append(r.env.fn.FreeVars, bdg)

				// TODO: must the freevar be defined in every enclosing function up to
				// the cell? Currently only in the function that references the cell.
				bdg = &Binding{
					Decl:  bdg.Decl,
					Const: bdg.Const,
					Scope: Free,
					Index: ix,
				}
				r.env.bindings[ident.Lit] = bdg
			}
			ident.Binding = bdg
			return
		}
	}

	// look for a predeclared or universal binding
	// TODO: should save those bindings in the r.env to shortcut subsequent lookups?
	if r.isPredeclared != nil && r.isPredeclared(ident.Lit) {
		bdg, ok := r.globals[ident.Lit]
		if !ok {
			bdg = &Binding{Scope: Predeclared, Decl: ident}
			r.globals[ident.Lit] = bdg
		}
		ident.Binding = bdg
		return
	}
	if r.isUniversal != nil && r.isUniversal(ident.Lit) {
		bdg, ok := r.globals[ident.Lit]
		if !ok {
			bdg = &Binding{Scope: Universal, Decl: ident}
			r.globals[ident.Lit] = bdg
		}
		ident.Binding = bdg
		return
	}
	r.errorf(ident.Start, "undefined: %s", ident.Lit)
}

func (r *resolver) useLabel(ident *ast.IdentExpr, requireLoopLabel bool) {
	// labels in current or any parent block are visible, but only inside the
	// current function, and not across defer/catch blocks (i.e. a break,
	// continue or goto in a defer cannot target a label outside that defer).
	curFn := r.env.fn
	for env := r.env; env != nil && env.fn == curFn; env = env.parent {
		bdg := env.bindings[ident.Lit]
		if bdg != nil {
			// binding found, must be a label, and may need to be associated with a
			// loop
			if bdg.Scope != Label && bdg.Scope != LoopLabel {
				r.errorf(ident.Start, "label %s not defined", ident.Lit)
				return
			}

			if requireLoopLabel && bdg.Scope != LoopLabel {
				r.errorf(ident.Start, "label %s not associated with a loop", ident.Lit)
				return
			}
			ident.Binding = bdg
			return
		}

		if env.isDeferCatch {
			break // cannot continue looking in parent block
		}
	}
	r.errorf(ident.Start, "label %s not defined", ident.Lit)
}

type block struct {
	parent *block // nil for file block
	fn     *Function

	// indicates if this is the top-level block of a defer or a catch, which
	// cannot "see" labels in the parent blocks.
	isDeferCatch bool

	// bindings maps a name to its binding. A local binding has an index
	// into its innermost enclosing function's locals array. A free
	// binding has an index into its innermost enclosing function's
	// freevars array.
	bindings map[string]*Binding

	// children records the child blocks of the current one.
	children []*block
}
