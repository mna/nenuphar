// Much of the compiler package is adapted from the Starlark source code:
// https://github.com/google/starlark-go/tree/ee8ed142361c69d52fe8e9fb5e311d2a0a7c02de
//
// Copyright 2017 The Bazel Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package compiler takes a parsed and resolved AST and compiles it to bytecode
// that can be executed by the virtual machine. It also provides a
// pseudo-assembly serialization and deserialization to encode in textual form
// a program that closely matches the binary format of the compiled form.
package compiler

import (
	"context"
	"fmt"

	"github.com/mna/nenuphar/lang/ast"
	"github.com/mna/nenuphar/lang/resolver"
	"github.com/mna/nenuphar/lang/token"
	"go.starlark.net/resolve"
)

// CompileFiles takes the file set and corresponding list of chunks from
// a successful resolve result and compiles the AST to bytecode.
//
// An AST that resulted in errors in the resolve phase should never be
// passed to the compiler, the behavior is undefined.
//
// Compiling files does not return an error as a valid resolved AST
// should always generate a valid, executable compiled program.
func CompileFiles(ctx context.Context, fset *token.FileSet, chunks []*ast.Chunk) []*Program {
	if len(chunks) == 0 {
		return nil
	}

	progs := make([]*Program, len(chunks))
	for i, ch := range chunks {
		start, _ := ch.Span()
		file := fset.File(start)
		pcomp := &pcomp{
			prog: &Program{
				Filename: file.Name(),
			},
			file:      file,
			names:     make(map[string]uint32),
			constants: make(map[interface{}]uint32),
			functions: make(map[*Funcode]uint32),
		}
		topLevel := pcomp.function(pcomp.prog.Filename, start, ch.Block, nil, nil)
		pcomp.prog.Functions[0] = topLevel
		progs[i] = pcomp.prog
	}
	return progs
}

// A pcomp holds the compiler state for a Program.
type pcomp struct {
	prog *Program    // what we're building
	file *token.File // to resolve token.Pos positions

	names     map[string]uint32
	constants map[interface{}]uint32
	functions map[*Funcode]uint32
}

func (pcomp *pcomp) function(name string, start token.Pos, block *ast.Block, locals, freevars []*resolver.Binding) *Funcode {
	fnPos := positionFromTokenPos(pcomp.file, start)
	fcomp := &fcomp{
		pcomp: pcomp,
		pos:   fnPos,
		fn: &Funcode{
			Prog:     pcomp.prog,
			pos:      fnPos,
			Name:     name,
			Locals:   bindings(pcomp.file, locals),
			Freevars: bindings(pcomp.file, freevars),
		},
	}

	// Record indices of locals that require cells.
	for i, local := range locals {
		if local.Scope == resolver.Cell {
			fcomp.fn.Cells = append(fcomp.fn.Cells, i)
		}
	}

	// Convert AST to a CFG of instructions.
	entry := fcomp.newBlock()
	fcomp.block = entry
	fcomp.stmts(block.Stmts)
	if fcomp.block != nil {
		fcomp.emit(NIL)
		fcomp.emit(RETURN)
	}

	/*
		var oops bool // something bad happened

		setinitialstack := func(b *block, depth int) {
			if b.initialstack == -1 {
				b.initialstack = depth
			} else if b.initialstack != depth {
				fmt.Fprintf(os.Stderr, "%d: setinitialstack: depth mismatch: %d vs %d\n",
					b.index, b.initialstack, depth)
				oops = true
			}
		}

		// Linearize the CFG:
		// compute order, address, and initial
		// stack depth of each reachable block.
		var pc uint32
		var blocks []*block
		var maxstack int
		var visit func(b *block)
		visit = func(b *block) {
			if b.index >= 0 {
				return // already visited
			}
			b.index = len(blocks)
			b.addr = pc
			blocks = append(blocks, b)

			stack := b.initialstack
			if debug {
				fmt.Fprintf(os.Stderr, "%s block %d: (stack = %d)\n", name, b.index, stack)
			}
			var cjmpAddr *uint32
			var isiterjmp int
			for i, insn := range b.insns {
				pc++

				// Compute size of argument.
				if insn.op >= OpcodeArgMin {
					switch insn.op {
					case ITERJMP:
						isiterjmp = 1
						fallthrough
					case CJMP:
						cjmpAddr = &b.insns[i].arg
						pc += 4
					default:
						pc += uint32(varArgLen(insn.arg))
					}
				}

				// Compute effect on stack.
				se := insn.stackeffect()
				if debug {
					fmt.Fprintln(os.Stderr, "\t", insn.op, stack, stack+se)
				}
				stack += se
				if stack < 0 {
					fmt.Fprintf(os.Stderr, "After pc=%d: stack underflow\n", pc)
					oops = true
				}
				if stack+isiterjmp > maxstack {
					maxstack = stack + isiterjmp
				}
			}

			// Place the jmp block next.
			if b.jmp != nil {
				// jump threading (empty cycles are impossible)
				for b.jmp.insns == nil {
					b.jmp = b.jmp.jmp
				}

				setinitialstack(b.jmp, stack+isiterjmp)
				if b.jmp.index < 0 {
					// Successor is not yet visited:
					// place it next and fall through.
					visit(b.jmp)
				} else {
					// Successor already visited;
					// explicit backward jump required.
					pc += 5
				}
			}

			// Then the cjmp block.
			if b.cjmp != nil {
				// jump threading (empty cycles are impossible)
				for b.cjmp.insns == nil {
					b.cjmp = b.cjmp.jmp
				}

				setinitialstack(b.cjmp, stack)
				visit(b.cjmp)

				// Patch the CJMP/ITERJMP, if present.
				if cjmpAddr != nil {
					*cjmpAddr = b.cjmp.addr
				}
			}
		}
		setinitialstack(entry, 0)
		visit(entry)
	*/

	fn := fcomp.fn
	fn.MaxStack = maxstack

	// Emit bytecode (and position table).
	fcomp.generate(blocks, pc)

	// Don't panic until we've completed printing of the function.
	if oops {
		panic("internal error")
	}

	return fn
}

// nameIndex returns the index of the specified name within the name pool,
// adding it if necessary.
func (pcomp *pcomp) nameIndex(name string) uint32 {
	index, ok := pcomp.names[name]
	if !ok {
		index = uint32(len(pcomp.prog.Names))
		pcomp.names[name] = index
		pcomp.prog.Names = append(pcomp.prog.Names, name)
	}
	return index
}

// constantIndex returns the index of the specified constant within the
// constant pool, adding it if necessary.
func (pcomp *pcomp) constantIndex(v interface{}) uint32 {
	index, ok := pcomp.constants[v]
	if !ok {
		index = uint32(len(pcomp.prog.Constants))
		pcomp.constants[v] = index
		pcomp.prog.Constants = append(pcomp.prog.Constants, v)
	}
	return index
}

// functionIndex returns the index of the specified function within the
// function pool, adding it if necessary.
func (pcomp *pcomp) functionIndex(fn *Funcode) uint32 {
	index, ok := pcomp.functions[fn]
	if !ok {
		index = uint32(len(pcomp.prog.Functions))
		pcomp.functions[fn] = index
		pcomp.prog.Functions = append(pcomp.prog.Functions, fn)
	}
	return index
}

// An fcomp holds the compiler state for a Funcode.
type fcomp struct {
	fn *Funcode // what we're building

	pcomp *pcomp
	pos   Position // current position of generated code (not necessarily == to fn.pos)
	loops []loop
	block *block
	// TODO(mna): probably needs to keep track of catch blocks during compilation?
}

// newBlock returns a new block.
func (fcomp) newBlock() *block {
	return &block{index: -1, initialstack: -1}
}

func (fcomp *fcomp) stmts(stmts []ast.Stmt) {
	for _, stmt := range stmts {
		fcomp.stmt(stmt)
	}
}

func (fcomp *fcomp) stmt(stmt ast.Stmt) {
	switch stmt := stmt.(type) {
	case *ast.ExprStmt:
		// compute the expression (will be a function call) and ignore the
		// resulting value (pop it off the stack)
		fcomp.expr(stmt.Expr)
		fcomp.emit(POP)

		/*
			case *syntax.BranchStmt:
				// Resolver invariant: break/continue appear only within loops.
				switch stmt.Token {
				case syntax.PASS:
					// no-op
				case syntax.BREAK:
					b := fcomp.loops[len(fcomp.loops)-1].break_
					fcomp.jump(b)
					fcomp.block = fcomp.newBlock() // dead code
				case syntax.CONTINUE:
					b := fcomp.loops[len(fcomp.loops)-1].continue_
					fcomp.jump(b)
					fcomp.block = fcomp.newBlock() // dead code
				}

			case *syntax.IfStmt:
				// Keep consistent with CondExpr.
				t := fcomp.newBlock()
				f := fcomp.newBlock()
				done := fcomp.newBlock()

				fcomp.ifelse(stmt.Cond, t, f)

				fcomp.block = t
				fcomp.stmts(stmt.True)
				fcomp.jump(done)

				fcomp.block = f
				fcomp.stmts(stmt.False)
				fcomp.jump(done)

				fcomp.block = done

			case *syntax.AssignStmt:
				switch stmt.Op {
				case syntax.EQ:
					// simple assignment: x = y
					fcomp.expr(stmt.RHS)
					fcomp.assign(stmt.OpPos, stmt.LHS)

				case syntax.PLUS_EQ,
					syntax.MINUS_EQ,
					syntax.STAR_EQ,
					syntax.SLASH_EQ,
					syntax.SLASHSLASH_EQ,
					syntax.PERCENT_EQ,
					syntax.AMP_EQ,
					syntax.PIPE_EQ,
					syntax.CIRCUMFLEX_EQ,
					syntax.LTLT_EQ,
					syntax.GTGT_EQ:
					// augmented assignment: x += y

					var set func()

					// Evaluate "address" of x exactly once to avoid duplicate side-effects.
					switch lhs := unparen(stmt.LHS).(type) {
					case *syntax.Ident:
						// x = ...
						fcomp.lookup(lhs)
						set = func() {
							fcomp.set(lhs)
						}

					case *syntax.IndexExpr:
						// x[y] = ...
						fcomp.expr(lhs.X)
						fcomp.expr(lhs.Y)
						fcomp.emit(DUP2)
						fcomp.setPos(lhs.Lbrack)
						fcomp.emit(INDEX)
						set = func() {
							fcomp.setPos(lhs.Lbrack)
							fcomp.emit(SETINDEX)
						}

					case *syntax.DotExpr:
						// x.f = ...
						fcomp.expr(lhs.X)
						fcomp.emit(DUP)
						name := fcomp.pcomp.nameIndex(lhs.Name.Name)
						fcomp.setPos(lhs.Dot)
						fcomp.emit1(ATTR, name)
						set = func() {
							fcomp.setPos(lhs.Dot)
							fcomp.emit1(SETFIELD, name)
						}

					default:
						panic(lhs)
					}

					fcomp.expr(stmt.RHS)

					// In-place x+=y and x|=y have special semantics:
					// the resulting x aliases the original x.
					switch stmt.Op {
					case syntax.PLUS_EQ:
						fcomp.setPos(stmt.OpPos)
						fcomp.emit(INPLACE_ADD)
					case syntax.PIPE_EQ:
						fcomp.setPos(stmt.OpPos)
						fcomp.emit(INPLACE_PIPE)
					default:
						fcomp.binop(stmt.OpPos, stmt.Op-syntax.PLUS_EQ+syntax.PLUS)
					}
					set()
				}

			case *syntax.DefStmt:
				fcomp.function(stmt.Function.(*resolve.Function))
				fcomp.set(stmt.Name)

			case *syntax.ForStmt:
				// Keep consistent with ForClause.
				head := fcomp.newBlock()
				body := fcomp.newBlock()
				tail := fcomp.newBlock()

				fcomp.expr(stmt.X)
				fcomp.setPos(stmt.For)
				fcomp.emit(ITERPUSH)
				fcomp.jump(head)

				fcomp.block = head
				fcomp.condjump(ITERJMP, tail, body)

				fcomp.block = body
				fcomp.assign(stmt.For, stmt.Vars)
				fcomp.loops = append(fcomp.loops, loop{break_: tail, continue_: head})
				fcomp.stmts(stmt.Body)
				fcomp.loops = fcomp.loops[:len(fcomp.loops)-1]
				fcomp.jump(head)

				fcomp.block = tail
				fcomp.emit(ITERPOP)

			case *syntax.WhileStmt:
				head := fcomp.newBlock()
				body := fcomp.newBlock()
				done := fcomp.newBlock()

				fcomp.jump(head)
				fcomp.block = head
				fcomp.ifelse(stmt.Cond, body, done)

				fcomp.block = body
				fcomp.loops = append(fcomp.loops, loop{break_: done, continue_: head})
				fcomp.stmts(stmt.Body)
				fcomp.loops = fcomp.loops[:len(fcomp.loops)-1]
				fcomp.jump(head)

				fcomp.block = done

			case *syntax.ReturnStmt:
				if stmt.Result != nil {
					fcomp.expr(stmt.Result)
				} else {
					fcomp.emit(NONE)
				}
				fcomp.emit(RETURN)
				fcomp.block = fcomp.newBlock() // dead code

			case *syntax.LoadStmt:
				for i := range stmt.From {
					fcomp.string(stmt.From[i].Name)
				}
				module := stmt.Module.Value.(string)
				fcomp.pcomp.prog.Loads = append(fcomp.pcomp.prog.Loads, Binding{
					Name: module,
					Pos:  stmt.Module.TokenPos,
				})
				fcomp.string(module)
				fcomp.setPos(stmt.Load)
				fcomp.emit1(LOAD, uint32(len(stmt.From)))
				for i := range stmt.To {
					fcomp.set(stmt.To[len(stmt.To)-1-i])
				}
		*/

	default:
		// TODO: use a central function to panic with position information
		panic(fmt.Sprintf("unexpected stmt %T", stmt))
	}
}

func (fcomp *fcomp) expr(e ast.Expr) {
	switch e := e.(type) {
	case *ast.ParenExpr:
		fcomp.expr(e.Expr)

	case *ast.IdentExpr:
		fcomp.lookup(e)

	case *ast.LiteralExpr:
		switch e.Type {
		case token.NULL:
			fcomp.emit(NIL)
		case token.TRUE:
			fcomp.emit(TRUE)
		case token.FALSE:
			fcomp.emit(FALSE)
		default:
			// e.Value is int64, float64, string
			v := e.Value
			fcomp.emit1(CONSTANT, fcomp.pcomp.constantIndex(v))
		}

	case *ast.ArrayLikeExpr:
		for _, v := range e.Items {
			fcomp.expr(v)
		}
		if e.Type == token.LBRACK {
			fcomp.emit1(MAKEARRAY, uint32(len(e.Items)))
		} else {
			fcomp.emit1(MAKETUPLE, uint32(len(e.Items)))
		}

	case *ast.DotExpr:
		fcomp.expr(e.Left)
		fcomp.setPos(e.Dot)
		fcomp.emit1(ATTR, fcomp.pcomp.nameIndex(e.Right.Lit))

	case *ast.IndexExpr:
		fcomp.expr(e.Prefix)
		fcomp.expr(e.Index)
		fcomp.setPos(e.Lbrack)
		fcomp.emit(INDEX)

	case *ast.MapExpr:
		fcomp.emit1(MAKEMAP, uint32(len(e.Items)))
		for _, kv := range e.Items {
			fcomp.emit(DUP)
			fcomp.expr(kv.Key)
			fcomp.expr(kv.Value)
			fcomp.setPos(kv.Colon)
			fcomp.emit(SETMAP)
		}

	case *ast.FuncExpr:
		fcomp.function(e.Function.(*resolver.Function))

		/*
			case *syntax.UnaryExpr:
				fcomp.expr(e.X)
				fcomp.setPos(e.OpPos)
				switch e.Op {
				case syntax.MINUS:
					fcomp.emit(UMINUS)
				case syntax.PLUS:
					fcomp.emit(UPLUS)
				case syntax.NOT:
					fcomp.emit(NOT)
				case syntax.TILDE:
					fcomp.emit(TILDE)
				default:
					log.Panicf("%s: unexpected unary op: %s", e.OpPos, e.Op)
				}

			case *syntax.CondExpr:
				// Keep consistent with IfStmt.
				t := fcomp.newBlock()
				f := fcomp.newBlock()
				done := fcomp.newBlock()

				fcomp.ifelse(e.Cond, t, f)

				fcomp.block = t
				fcomp.expr(e.True)
				fcomp.jump(done)

				fcomp.block = f
				fcomp.expr(e.False)
				fcomp.jump(done)

				fcomp.block = done

			case *syntax.SliceExpr:
				fcomp.setPos(e.Lbrack)
				fcomp.expr(e.X)
				if e.Lo != nil {
					fcomp.expr(e.Lo)
				} else {
					fcomp.emit(NONE)
				}
				if e.Hi != nil {
					fcomp.expr(e.Hi)
				} else {
					fcomp.emit(NONE)
				}
				if e.Step != nil {
					fcomp.expr(e.Step)
				} else {
					fcomp.emit(NONE)
				}
				fcomp.emit(SLICE)

			case *syntax.Comprehension:
				if e.Curly {
					fcomp.emit(MAKEDICT)
				} else {
					fcomp.emit1(MAKELIST, 0)
				}
				fcomp.comprehension(e, 0)

			case *syntax.TupleExpr:
				fcomp.tuple(e.List)

			case *syntax.BinaryExpr:
				switch e.Op {
				// short-circuit operators
				// TODO(adonovan): use ifelse to simplify conditions.
				case syntax.OR:
					// x or y  =>  if x then x else y
					done := fcomp.newBlock()
					y := fcomp.newBlock()

					fcomp.expr(e.X)
					fcomp.emit(DUP)
					fcomp.condjump(CJMP, done, y)

					fcomp.block = y
					fcomp.emit(POP) // discard X
					fcomp.expr(e.Y)
					fcomp.jump(done)

					fcomp.block = done

				case syntax.AND:
					// x and y  =>  if x then y else x
					done := fcomp.newBlock()
					y := fcomp.newBlock()

					fcomp.expr(e.X)
					fcomp.emit(DUP)
					fcomp.condjump(CJMP, y, done)

					fcomp.block = y
					fcomp.emit(POP) // discard X
					fcomp.expr(e.Y)
					fcomp.jump(done)

					fcomp.block = done

				case syntax.PLUS:
					fcomp.plus(e)

				default:
					// all other strict binary operator (includes comparisons)
					fcomp.expr(e.X)
					fcomp.expr(e.Y)
					fcomp.binop(e.OpPos, e.Op)
				}

			case *syntax.CallExpr:
				fcomp.call(e)

			case *syntax.LambdaExpr:
				fcomp.function(e.Function.(*resolve.Function))
		*/

	default:
		panic(fmt.Sprintf("unexpected expr %T", e))
	}
}

func (fcomp *fcomp) function(f *resolver.Function) {
	// MAKEFUNC does not fail, no need to record position. It expects a tuple of
	// freevars on the stack and takes the index of the function as argument.

	// Capture the cells of the function's free variables from the lexical
	// environment.
	for _, freevar := range f.FreeVars {
		// Don't call fcomp.lookup because we want the cell itself, not its
		// content.
		switch freevar.Scope {
		case resolve.Free:
			fcomp.emit1(FREE, uint32(freevar.Index))
		case resolve.Cell:
			fcomp.emit1(LOCAL, uint32(freevar.Index))
		}
	}
	fcomp.emit1(MAKETUPLE, uint32(len(f.FreeVars)))
	start, _ := f.Definition.Span()
	funcode := fcomp.pcomp.function(f.Name, start, f.Body, f.Locals, f.FreeVars)

	numParams := len(f.Params)
	if f.HasVarArg {
		numParams--
	}

	funcode.NumParams = numParams
	funcode.HasVarArg = f.HasVarArg
	fcomp.emit1(MAKEFUNC, fcomp.pcomp.functionIndex(funcode))
}

// lookup emits code to push the value of the specified variable.
func (fcomp *fcomp) lookup(id *ast.IdentExpr) {
	bind := id.Binding.(*resolver.Binding)
	if bind.Scope != resolver.Universal { // (universal lookup can't fail)
		fcomp.setPos(id.Start)
	}
	switch bind.Scope {
	case resolver.Local:
		fcomp.emit1(LOCAL, uint32(bind.Index))
	case resolver.Free:
		fcomp.emit1(FREECELL, uint32(bind.Index))
	case resolver.Cell:
		fcomp.emit1(LOCALCELL, uint32(bind.Index))
	case resolver.Predeclared:
		fcomp.emit1(PREDECLARED, fcomp.pcomp.nameIndex(id.Lit))
	case resolver.Universal:
		fcomp.emit1(UNIVERSAL, fcomp.pcomp.nameIndex(id.Lit))
	default:
		panic(fmt.Sprintf("%s: compiler.lookup(%s): scope = %d", positionFromTokenPos(fcomp.pcomp.file, id.Start), id.Lit, bind.Scope))
	}
}

// emit emits an instruction to the current block.
func (fcomp *fcomp) emit(op Opcode) {
	if op >= OpcodeArgMin {
		panic("missing argument for opcode " + op.String())
	}
	insn := insn{op: op, line: fcomp.pos.Line, col: fcomp.pos.Col}
	fcomp.block.insns = append(fcomp.block.insns, insn)
	fcomp.pos.Line = 0
	fcomp.pos.Col = 0
}

// emit1 emits an instruction with an immediate operand.
func (fcomp *fcomp) emit1(op Opcode, arg uint32) {
	if op < OpcodeArgMin {
		panic("unwanted arg: " + op.String())
	}
	insn := insn{op: op, arg: arg, line: fcomp.pos.Line, col: fcomp.pos.Col}
	fcomp.block.insns = append(fcomp.block.insns, insn)
	fcomp.pos.Line = 0
	fcomp.pos.Col = 0
}

// setPos sets the current source position, it should be called prior to any
// operation that can fail dynamically (in the machine interpreter). All positions are assumed to belong to
// fcomp.pcomp.file.
func (fcomp *fcomp) setPos(pos token.Pos) {
	fcomp.pos = positionFromTokenPos(fcomp.pcomp.file, pos)
}

type loop struct {
	break_, continue_ *block
}

// block is a block of code - every executable line of code is compiled inside
// a block.
type block struct {
	insns []insn

	// If the last insn is a RETURN, jmp and cjmp are nil.
	// If the last insn is a CJMP or ITERJMP,
	//  cjmp and jmp are the "true" and "false" successors.
	// Otherwise, jmp is the sole successor.
	jmp, cjmp *block

	initialstack int // for stack depth computation

	// Used during encoding
	index int // -1 => not encoded yet
	addr  uint32
}

// bindings converts resolver.Bindings to compiled form.
func bindings(file *token.File, bindings []*resolver.Binding) []Binding {
	res := make([]Binding, len(bindings))
	for i, bind := range bindings {
		res[i].Name = bind.Decl.Lit
		res[i].Pos = positionFromTokenPos(file, bind.Decl.Start)
	}
	return res
}

type insn struct {
	op        Opcode
	arg       uint32
	line, col uint32
}

func encodeInsn(code []byte, op Opcode, arg uint32) []byte {
	code = append(code, byte(op))
	if op >= OpcodeArgMin {
		if isJump(op) {
			code = addUint32(code, arg, 4) // pad arg to 4 bytes
		} else {
			code = addUint32(code, arg, 0)
		}
	}
	return code
}

// addUint32 encodes x as 7-bit little-endian varint.
func addUint32(code []byte, x uint32, min int) []byte {
	end := len(code) + min
	for x >= 0x80 {
		code = append(code, byte(x)|0x80)
		x >>= 7
	}
	code = append(code, byte(x))
	// Pad the operand with NOPs to exactly min bytes.
	for len(code) < end {
		code = append(code, byte(NOP))
	}
	return code
}
