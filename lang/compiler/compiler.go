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

	"github.com/mna/nenuphar/lang/ast"
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
			names:     make(map[string]uint32),
			constants: make(map[interface{}]uint32),
			functions: make(map[*Funcode]uint32),
		}
		topLevel := pcomp.function(name, pos, stmts, locals, nil)
		pcomp.prog.Functions[0] = topLevel
		progs[i] = pcomp.prog
	}
	return progs
}

// A pcomp holds the compiler state for a Program.
type pcomp struct {
	prog *Program // what we're building

	names     map[string]uint32
	constants map[interface{}]uint32
	functions map[*Funcode]uint32
}

// An fcomp holds the compiler state for a Funcode.
type fcomp struct {
	fn *Funcode // what we're building

	pcomp *pcomp
	pos   token.Position // current position of generated code (TODO: token.Pos?)
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
