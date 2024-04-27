// Much of the resolver package is adapted from the Starlark source code:
// https://github.com/google/starlark-go/tree/ee8ed142361c69d52fe8e9fb5e311d2a0a7c02de
//
// Copyright 2017 The Bazel Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

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
//   - BindGuardStmt: e.g. "guard let x = 1 else .. end". The scope of the
//     bindings is the enclosing block of the guard statement (from that point
//     on).
//   - ThreePartForStmt: e.g. "for let x = 1; .. end". The scope of the
//     bindings are limited to the body of the "for" loop.
//   - ForInStmt: e.g. "for x, y, x in .. end". The scope of the bindings are
//     limited to the body of the "for" loop. New bindings are always defined for
//     this syntax when identifiers are used (implicit "let").
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
//   - MethodDef: e.g. "fn Bar() .. end" inside a class. Visible to all class
//     methods.
//   - FieldDef: e.g. "let x = 1" inside a class. Visible to subsequent fields
//     and all methods.
package resolver

import (
	"context"
	"fmt"

	"github.com/mna/nenuphar/lang/ast"
	"github.com/mna/nenuphar/lang/scanner"
	"github.com/mna/nenuphar/lang/token"
)

// Mode is a set of bit flags that configures the resolving. By default (0),
// the symbols are resolved, all errors are reported and blocks are not given
// unique names.
type Mode uint

// List of supported resolver modes, which can be combined with bitwise or.
const (
	NameBlocks Mode = 1 << iota // give unique names to blocks, useful for printing the resolved AST.
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
	mode Mode, isPredeclared, isUniversal func(name string) bool) error {
	if len(chunks) == 0 {
		return nil
	}

	var r resolver
	r.isPredeclared = isPredeclared
	if isPredeclared == nil {
		r.isPredeclared = func(name string) bool { return false }
	}
	r.isUniversal = isUniversal
	if isUniversal == nil {
		r.isUniversal = func(name string) bool { return false }
	}

	for _, ch := range chunks {
		start, _ := ch.Span()
		r.init(fset.File(start))
		r.block(ch.Block, ch)

		if mode&NameBlocks != 0 {
			// assign all names in one go at the end, so that performance is not
			// impacted at all if this option is not set.
			r.nameBlocks()
		}
	}
	r.errors.Sort()
	return r.errors.Err()
}

type resolver struct {
	file   *token.File
	errors scanner.ErrorList

	// env is the current local environment, a linked list of blocks, with the
	// current innermost block first and the tail of the list the file
	// (top-level) block.
	env *block
	// root keeps a reference to the root block
	root *block

	// globals saves the bindings of predeclared and universal names when they
	// are first referenced.
	globals map[string]*Binding

	// predicates to check if an unresolved name is predeclared or universal.
	isPredeclared, isUniversal func(name string) bool
}

func (r *resolver) init(file *token.File) {
	r.file = file
	r.env = nil
	r.root = nil
	r.globals = make(map[string]*Binding)
}

func (r *resolver) push(b *block) {
	if r.env == nil {
		// this is the root block
		r.root = b
	} else {
		r.env.children = append(r.env.children, b)
		if b.fn == nil {
			// in same function as before
			b.fn = r.env.fn
		}
	}
	b.parent = r.env
	b.bindings = make(map[string]*Binding)
	r.env = b
}

func (r *resolver) pop() {
	r.env = r.env.parent
}

func (r *resolver) errorf(p token.Pos, format string, args ...interface{}) {
	r.errors.Add(r.file.Position(p), fmt.Sprintf(format, args...))
}

func (r *resolver) block(b *ast.Block, from ast.Node) {
	var (
		blk     block
		isLoop  bool
		isDefer bool
		isCatch bool
	)

	switch v := from.(type) {
	case *ast.Chunk:
		blk.fn = &Function{Definition: v}
	case *ast.SimpleBlockStmt:
		isDefer = v.Type == token.DEFER
		isCatch = v.Type == token.CATCH
		blk.isDeferCatch = isDefer || isCatch
	case ast.Stmt:
		isLoop = v.IsLoop()
	}

	r.push(&blk)
	switch {
	case isLoop:
		blk.fn.loops++
	case isDefer:
		blk.fn.defers++
	case isCatch:
		blk.fn.catches++
	}

	for _, s := range b.Stmts {
		r.stmt(s)
	}

	r.pop()
	switch {
	case isLoop:
		blk.fn.loops--
	case isDefer:
		blk.fn.defers--
	case isCatch:
		blk.fn.catches--
	}
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
		// resolve the inherits clause first
		if stmt.Inherits != nil && stmt.Inherits.Expr != nil {
			r.expr(stmt.Inherits.Expr)
		}

		// bind the name before the body, as it can be used by itself
		// TODO: double-check that this works once machine is implemented
		r.bind(stmt.Name, true)
		r.class(stmt, stmt.Body)

	case *ast.ExprStmt:
		r.expr(stmt.Expr)

	case *ast.ForInStmt:
		// resolve the rhs first
		for _, e := range stmt.Right {
			r.expr(e)
		}

		// lhs are implicit declarations if identifiers, otherwise must be resolved
		// TODO: that's not super clean, think about using for let x, y in z()..
		// instead.
		var toBind []*ast.IdentExpr
		for _, e := range stmt.Left {
			if id, ok := e.(*ast.IdentExpr); ok {
				toBind = append(toBind, id)
			} else {
				r.expr(e)
			}
		}
		// if there are loop-scoped identifiers, create a synthetic block to hold them
		if len(toBind) > 0 {
			r.push(new(block))
			for _, e := range toBind {
				r.bind(e, false)
			}
		}
		r.block(stmt.Body, stmt)
		if len(toBind) > 0 {
			r.pop()
		}

	case *ast.ForLoopStmt:
		// everything in the 3-part for loop is in a synthetic block around the
		// body, so if the init part declares any variables, they are scoped to the
		// loop. Cond and Post may use the Init-declared variables.
		r.push(new(block))

		if stmt.Init != nil {
			r.stmt(stmt.Init)
		}
		if stmt.Cond != nil {
			r.expr(stmt.Cond)
		}
		if stmt.Post != nil {
			r.stmt(stmt.Post)
		}
		r.block(stmt.Body, stmt)

		r.pop()

	case *ast.FuncStmt:
		r.bind(stmt.Name, true)
		r.function(stmt, stmt.Sig, stmt.Body)

	case *ast.IfGuardStmt:
		// regardless of whether this is an if, elseif or guard, the condition
		// resolves in the enclosing environment.
		if stmt.Cond != nil {
			r.expr(stmt.Cond)
			if stmt.True != nil {
				r.block(stmt.True, stmt)
			}
			if stmt.False != nil {
				// do not create a new block for an elseif, process it as an if
				if len(stmt.False.Stmts) == 1 {
					if ifst, ok := stmt.False.Stmts[0].(*ast.IfGuardStmt); ok {
						if ifst.Type == token.ELSEIF {
							r.stmt(ifst)
							break
						}
					}
				}
				// otherwise create a block for the false block
				r.block(stmt.False, stmt)
			}
			break
		}

		// if this is a declaration (if-bind or guard-bind), the rhs resolves in the
		// enclosing environment, the lhs is defined inside the if-true block for if,
		// and in the enclosing environment (but _after_ the false block) for guard.
		if stmt.Decl != nil {
			for _, e := range stmt.Decl.Right {
				r.expr(e)
			}

			switch stmt.Type {
			case token.GUARD:
				// first resolve the false block
				r.block(stmt.False, stmt)
				// then define the lhs of the declaration in the enclosing block
				for _, e := range stmt.Decl.Left {
					r.bind(e.(*ast.IdentExpr), stmt.Decl.DeclType == token.CONST)
				}

			case token.IF, token.ELSEIF:
				// define the lhs of the declaration in the true block (in a synthetic
				// block that only encloses the true block)
				r.push(new(block))
				for _, e := range stmt.Decl.Left {
					r.bind(e.(*ast.IdentExpr), stmt.Decl.DeclType == token.CONST)
				}
				r.block(stmt.True, stmt)
				r.pop()

				if stmt.False != nil {
					// do not create a new block for an elseif, process it as an if
					if len(stmt.False.Stmts) == 1 {
						if ifst, ok := stmt.False.Stmts[0].(*ast.IfGuardStmt); ok {
							if ifst.Type == token.ELSEIF {
								r.stmt(ifst)
								break
							}
						}
					}
					// otherwise create a block for the false block
					r.block(stmt.False, stmt)
				}

			default:
				panic(fmt.Sprintf("unexpected if statement type: %v", stmt.Type))
			}
		}

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
				r.errorf(stmt.Start, "invalid return inside defer block")
			}
		} else if stmt.Type == token.THROW {
			if stmt.Expr == nil && r.env.fn.catches == 0 {
				r.errorf(stmt.Start, "invalid re-throw: not inside a catch block")
			}
		}

		if stmt.Expr != nil {
			r.expr(stmt.Expr)
		}

	case *ast.SimpleBlockStmt:
		r.block(stmt.Body, stmt)

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
		if expr.Inherits != nil && expr.Inherits.Expr != nil {
			r.expr(expr.Inherits.Expr)
		}
		r.class(expr, expr.Body)

	case *ast.DotExpr:
		// ignore right, can be anything (runtime lookup)
		r.expr(expr.Left)

	case *ast.FuncExpr:
		r.function(expr, expr.Sig, expr.Body)

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

func (r *resolver) function(fn ast.Node, sig *ast.FuncSignature, body *ast.Block) {
	// bind the parameters in the function's block (in a synthetic block that
	// only encloses the function body)
	blk := &block{
		fn: &Function{
			Definition: fn,
			HasVarArg:  sig.DotDotDot.IsValid(),
		},
	}
	r.push(blk)
	for _, e := range sig.Params {
		r.bind(e, false)
	}
	r.block(body, fn)
	r.pop()
}

func (r *resolver) class(cl ast.Node, body *ast.ClassBody) {
	// all class members are scoped to the class's body, but we don't call
	// r.block() as we have some special processing of the fields and methods
	// to do.
	blk := &block{fn: &Function{Definition: cl}}
	r.push(blk)

	// fields get declared first, they are all available to methods and to
	// subsequent fields.
	for _, f := range body.Fields {
		// resolve the rhs of the declarations first, which cannot refer to methods
		for _, e := range f.Right {
			r.expr(e)
		}

		for _, e := range f.Left {
			r.bind(e.(*ast.IdentExpr), f.DeclType == token.CONST)
		}
	}

	// methods get declared next, they are visible to all other methods
	// regardless of order of declaration.
	for _, m := range body.Methods {
		r.bind(m.Name, true)
	}
	// finally, resolve the methods' bodies
	for _, m := range body.Methods {
		r.function(m, m.Sig, m.Body)
	}

	r.pop()
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
	// rule: labels cannot be shadowed, and a label cannot shadow a variable
	// inside the scope frontiers of the label.
	curFn := r.env.fn
	for env := r.env; env != nil && env.fn == curFn; env = env.parent {
		bdg := env.bindings[ident.Lit]
		if bdg != nil {
			if env == r.env {
				r.errorf(ident.Start, "already declared in this block: %s", ident.Lit)
			} else {
				r.errorf(ident.Start, "already declared in an outer block: %s", ident.Lit)
			}
			return
		}

		if env.isDeferCatch {
			break // defer/catch is a scope frontier for labels
		}
	}

	// TODO: add validation that label is not a target in a local variable
	// declaration
	scope := Label
	if loop {
		scope = LoopLabel
	}
	bdg := &Binding{Scope: scope, Decl: ident}
	ix := len(r.env.fn.Labels)
	bdg.Index = ix
	r.env.fn.Labels = append(r.env.fn.Labels, bdg)
	r.env.bindings[ident.Lit] = bdg

	ident.Binding = bdg
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
	if r.isPredeclared(ident.Lit) {
		bdg, ok := r.globals[ident.Lit]
		if !ok {
			bdg = &Binding{Scope: Predeclared, Decl: ident}
			r.globals[ident.Lit] = bdg
		}
		ident.Binding = bdg
		return
	}
	if r.isUniversal(ident.Lit) {
		bdg, ok := r.globals[ident.Lit]
		if !ok {
			bdg = &Binding{Scope: Universal, Decl: ident}
			r.globals[ident.Lit] = bdg
		}
		ident.Binding = bdg
		return
	}

	// TODO: maybe add a spell checker? (did you mean...)
	r.errorf(ident.Start, "undefined: %s", ident.Lit)
	ident.Binding = &Binding{Scope: Undefined}
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
	ident.Binding = &Binding{Scope: Undefined}
}
