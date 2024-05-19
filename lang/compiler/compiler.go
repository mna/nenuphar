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
	"os"

	"github.com/mna/nenuphar/lang/ast"
	"github.com/mna/nenuphar/lang/resolver"
	"github.com/mna/nenuphar/lang/token"
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
	fcomp.stmts(stmts)
	if fcomp.block != nil {
		fcomp.emit(NONE)
		fcomp.emit(RETURN)
	}

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

// An fcomp holds the compiler state for a Funcode.
type fcomp struct {
	fn *Funcode // what we're building

	pcomp *pcomp
	pos   Position // current position of generated code (not necessarily == to fn.pos)
	loops []loop
	block *block
	// TODO(mna): probably needs to keep track of catch blocks during compilation?
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
	line, col int32
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
