package compiler

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

// This asm file implements a human-readable/writable form of a compiled
// program. This is mostly to support testing of the VM without going through
// the parsing and name resolution phases of a higher-level language. A
// disassembler is also implemented.
//
// The assembly format looks like this (indentation and spacing is arbitrary,
// but order of sections is important):
//
// 	program:                             # required
// 		loads:                             # optional, list of Loads
// 			name_of_load
// 		names:														 # optional, list of Names (attr/predeclared/universe)
//      fail
// 		constants:                         # optional, list of Constants
// 			string "abc"
// 			int    1234
// 			float  1.34
//
// 	function: NAME <stack> <params> +varargs
//                                       # required at least once for top-level
//  	locals:                            # optional, list of Locals
// 			x
//  	cells:                             # optional, name in Locals that require cells
// 			x
// 		freevars:                          # optional, list of Freevars
// 			y
// 		defers:                            # optional, list of Defer blocks
// 			10 20 5                          # index of pc0-pc1 and startpc in code section (will be translated to pc address)
// 		catches:                           # optional, list of Catch blocks
// 			10 20 5                          # index of pc0-pc1 and startpc in code section (will be translated to pc address)
// 		code:                              # required, list of instructions
//			NOP
// 			JMP 3                            # jump argument refers to index in code section (will be translated to pc address)
// 			CALL 2

var sections = map[string]bool{
	"program:":   true,
	"loads:":     true,
	"names:":     true,
	"constants:": true,
	"function:":  true,
	"locals:":    true,
	"cells:":     true,
	"freevars:":  true,
	"defers:":    true,
	"catches:":   true,
	"code:":      true,
}

// Asm loads a compiled program from its assembler textual format.
func Asm(b []byte) (*Program, error) {
	asm := asm{s: bufio.NewScanner(bytes.NewReader(b))}

	// must start with the program: section
	fields := asm.next()
	asm.program(fields)

	// optional sections
	fields = asm.next()
	fields = asm.loads(fields)
	fields = asm.names(fields)
	fields = asm.constants(fields)

	// functions
	for asm.err == nil && len(fields) > 0 && fields[0] == "function:" {
		fields = asm.function(fields)
	}

	if asm.err == nil {
		if len(fields) > 0 {
			asm.err = fmt.Errorf("unexpected section: %s", fields[0])
		} else if asm.p.Toplevel == nil {
			asm.err = errors.New("missing top-level function")
		}
	}
	return asm.p, asm.err
}

type asm struct {
	s       *bufio.Scanner
	rawLine string // current raw line (not split in fields)
	p       *Program
	fn      *Funcode // current function
	err     error
}

func (a *asm) function(fields []string) []string {
	if a.err != nil || len(fields) == 0 || !strings.EqualFold(fields[0], "function:") {
		return fields
	}

	if len(fields) < 4 {
		a.err = fmt.Errorf("invalid function: want at least 4 fields: 'function: NAME <stack> <params> [+varargs]', got %d fields (%s)", len(fields), strings.Join(fields, " "))
		// force going forward, otherwise it would still process that line
		fields = a.next()
		return fields
	}
	fn := Funcode{
		Prog:       a.p,
		Name:       fields[1],
		MaxStack:   int(a.int(fields[2])),
		NumParams:  int(a.int(fields[3])),
		HasVarargs: a.option(fields[4:], "varargs"),
	}
	a.fn = &fn

	// function sub-sections
	fields = a.next()
	fields = a.locals(fields)
	fields = a.cells(fields)
	fields = a.freevars(fields)
	fields = a.defers(fields)
	fields = a.catches(fields)
	fields, indexToAddr := a.code(fields)

	if a.err == nil {
		// resolve the defer and catch addresses
		if err := resolveDefers(indexToAddr, a.fn.Defers, "defer"); err != nil {
			a.err = err
			return fields
		}
		if err := resolveDefers(indexToAddr, a.fn.Catches, "catch"); err != nil {
			a.err = err
			return fields
		}
	}

	a.fn = nil
	if a.p.Toplevel == nil {
		a.p.Toplevel = &fn
	} else {
		a.p.Functions = append(a.p.Functions, &fn)
	}
	return fields
}

func resolveDefers(indexToAddr []int, defers []Defer, label string) error {
	for i, d := range defers {
		if d.PC0 >= uint32(len(indexToAddr)) {
			return fmt.Errorf("invalid PC0 index %d: %s at index %d", d.PC0, label, i)
		}
		d.PC0 = uint32(indexToAddr[d.PC0])

		if d.PC1 >= uint32(len(indexToAddr)) {
			return fmt.Errorf("invalid PC1 index %d: %s at index %d", d.PC1, label, i)
		}
		d.PC1 = uint32(indexToAddr[d.PC1])

		if d.StartPC >= uint32(len(indexToAddr)) {
			return fmt.Errorf("invalid StartPC index %d: %s at index %d", d.StartPC, label, i)
		}
		d.StartPC = uint32(indexToAddr[d.StartPC])
		defers[i] = d
	}
	return nil
}

// parses code section and translates jump addresses to addresses, returning
// both the next fields to parse and the mapping of instruction index in the
// code section to address in the encoded code slice.
func (a *asm) code(fields []string) ([]string, []int) {
	var indexToAddr []int
	if a.err != nil {
		return fields, indexToAddr
	}
	if len(fields) == 0 || !strings.EqualFold(fields[0], "code:") {
		msg := "expected code section"
		if len(fields) > 0 {
			msg += ", found " + fields[0]
		}
		a.err = errors.New(msg)
		return fields, indexToAddr
	}

	var insns []insn
	var addr int
	for fields = a.next(); len(fields) > 0 && !sections[fields[0]]; fields = a.next() {
		op, ok := reverseLookupOpcode[strings.ToLower(fields[0])]
		if !ok {
			a.err = fmt.Errorf("invalid opcode: %s", fields[0])
			return fields, indexToAddr
		}

		var arg uint32
		if op >= OpcodeArgMin {
			// an argument is required
			if len(fields) != 2 {
				a.err = fmt.Errorf("expected an argument for opcode %s, got %d fields", fields[0], len(fields))
				return fields, indexToAddr
			}
			arg = uint32(a.uint(fields[1]))
		} else if len(fields) != 1 {
			a.err = fmt.Errorf("expected no argument for opcode %s, got %d fields", fields[0], len(fields))
			return fields, indexToAddr
		}
		insns = append(insns, insn{op: op, arg: arg})
		indexToAddr = append(indexToAddr, addr)
		addr += encodedSize(op, arg)
	}

	// encode the instructions with the translated addresses
	for i, insn := range insns {
		op, arg := insn.op, insn.arg
		if isJump(op) {
			if arg >= uint32(len(indexToAddr)) {
				a.err = fmt.Errorf("invalid jump index %d: instruction %s at index %d", arg, op, i)
				return fields, indexToAddr
			}
			arg = uint32(indexToAddr[arg])
		}
		a.fn.Code = encodeInsn(a.fn.Code, op, arg)
	}

	return fields, indexToAddr
}

func (a *asm) defers(fields []string) []string {
	if a.err != nil || len(fields) == 0 || !strings.EqualFold(fields[0], "defers:") {
		return fields
	}

	for fields = a.next(); len(fields) > 0 && !sections[fields[0]]; fields = a.next() {
		if len(fields) != 3 {
			a.err = fmt.Errorf("invalid defer: expected pc0, pc1 and startpc, got %d fields", len(fields))
			return fields
		}

		a.fn.Defers = append(a.fn.Defers, Defer{
			PC0:     uint32(a.uint(fields[0])),
			PC1:     uint32(a.uint(fields[1])),
			StartPC: uint32(a.uint(fields[2])),
		})
	}
	return fields
}

func (a *asm) catches(fields []string) []string {
	if a.err != nil || len(fields) == 0 || !strings.EqualFold(fields[0], "catches:") {
		return fields
	}

	for fields = a.next(); len(fields) > 0 && !sections[fields[0]]; fields = a.next() {
		if len(fields) != 3 {
			a.err = fmt.Errorf("invalid catch: expected pc0, pc1 and startpc, got %d fields", len(fields))
			return fields
		}

		a.fn.Catches = append(a.fn.Catches, Defer{
			PC0:     uint32(a.uint(fields[0])),
			PC1:     uint32(a.uint(fields[1])),
			StartPC: uint32(a.uint(fields[2])),
		})
	}
	return fields
}

func (a *asm) freevars(fields []string) []string {
	if a.err != nil || len(fields) == 0 || !strings.EqualFold(fields[0], "freevars:") {
		return fields
	}

	for fields = a.next(); len(fields) > 0 && !sections[fields[0]]; fields = a.next() {
		a.fn.Freevars = append(a.fn.Freevars, Binding{Name: fields[0]})
	}
	return fields
}

func (a *asm) cells(fields []string) []string {
	if a.err != nil || len(fields) == 0 || !strings.EqualFold(fields[0], "cells:") {
		return fields
	}

outer:
	for fields = a.next(); len(fields) > 0 && !sections[fields[0]]; fields = a.next() {
		for i, l := range a.fn.Locals {
			if l.Name == fields[0] {
				a.fn.Cells = append(a.fn.Cells, i)
				continue outer
			}
		}
		a.err = fmt.Errorf("invalid cell: %q is not an existing local", fields[0])
		return fields
	}
	return fields
}

func (a *asm) locals(fields []string) []string {
	if a.err != nil || len(fields) == 0 || !strings.EqualFold(fields[0], "locals:") {
		return fields
	}

	for fields = a.next(); len(fields) > 0 && !sections[fields[0]]; fields = a.next() {
		a.fn.Locals = append(a.fn.Locals, Binding{Name: fields[0]})
	}
	return fields
}

var rxConstLineString = regexp.MustCompile(`^\s*(?:string|bytes)\s+(.+)$`)

func (a *asm) constants(fields []string) []string {
	if a.err != nil || len(fields) == 0 || !strings.EqualFold(fields[0], "constants:") {
		return fields
	}

	for fields = a.next(); len(fields) > 0 && !sections[fields[0]]; fields = a.next() {
		// string and bytes constants may have whitespace in the value, need to
		// keep the raw line around and extract the whole quoted value from the raw
		// line.
		strVal := rxConstLineString.FindStringSubmatch(a.rawLine)
		if strVal == nil && len(fields) != 2 {
			a.err = fmt.Errorf("invalid constant: expected type and value, got %d fields", len(fields))
			return fields
		}

		switch fields[0] {
		case "int":
			a.p.Constants = append(a.p.Constants, a.int(fields[1]))
		case "float":
			f, err := strconv.ParseFloat(fields[1], 64)
			if err != nil {
				a.err = fmt.Errorf("invalid float: %s: %w", fields[1], err)
				return fields
			}
			a.p.Constants = append(a.p.Constants, f)
		case "string":
			qs, err := strconv.QuotedPrefix(strVal[1])
			if err != nil {
				a.err = fmt.Errorf("invalid string: %q: %w", strVal[1], err)
				return fields
			}
			s, err := strconv.Unquote(qs)
			if err != nil {
				a.err = fmt.Errorf("invalid string: %q: %w", qs, err)
				return fields
			}
			a.p.Constants = append(a.p.Constants, s)
		default:
			a.err = fmt.Errorf("invalid constant type: %s", fields[0])
			return fields
		}
	}
	return fields
}

func (a *asm) names(fields []string) []string {
	if a.err != nil || len(fields) == 0 || !strings.EqualFold(fields[0], "names:") {
		return fields
	}

	for fields = a.next(); len(fields) > 0 && !sections[fields[0]]; fields = a.next() {
		a.p.Names = append(a.p.Names, fields[0])
	}
	return fields
}

func (a *asm) loads(fields []string) []string {
	if a.err != nil || len(fields) == 0 || !strings.EqualFold(fields[0], "loads:") {
		return fields
	}

	for fields = a.next(); len(fields) > 0 && !sections[fields[0]]; fields = a.next() {
		a.p.Loads = append(a.p.Loads, Binding{Name: fields[0]})
	}
	return fields
}

func (a *asm) program(fields []string) {
	if a.err != nil {
		return
	}
	if len(fields) == 0 || !strings.EqualFold(fields[0], "program:") {
		msg := "expected program section"
		if len(fields) > 0 {
			msg += ", found " + fields[0]
		}
		a.err = errors.New(msg)
		return
	}

	var p Program
	a.p = &p
}

func (a *asm) option(fields []string, opt string) bool {
	for _, fld := range fields {
		if fld == "+"+opt {
			return true
		}
		if fld == "-"+opt {
			break
		}
	}
	return false
}

func (a *asm) int(s string) int64 {
	i, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		a.err = fmt.Errorf("invalid integer: %s: %w", s, err)
	}
	return i
}

func (a *asm) uint(s string) uint64 {
	u, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		a.err = fmt.Errorf("invalid unsigned integer: %s: %w", s, err)
	}
	return u
}

// returns the fields for the next non-empty, non-comment-only line, so that
// fields[0] will contain the line identification if it is a section.
func (a *asm) next() []string {
	a.rawLine = ""
	if a.err != nil {
		return nil
	}
	for a.s.Scan() {
		line := a.s.Text()
		fields := strings.Fields(line)
		if len(fields) != 0 && !strings.HasPrefix(fields[0], "#") {
			// strip comments to make rest of parsing simpler
			for i, fld := range fields {
				if strings.HasPrefix(fld, "#") {
					fields = fields[:i]
					break
				}
			}
			a.rawLine = line
			return fields
		}
	}
	a.err = a.s.Err()
	return nil
}

// Dasm writes a compiled program to its assembler textual format.
func Dasm(p *Program) ([]byte, error) {
	d := dasm{p: p, buf: new(bytes.Buffer)}
	d.program()
	d.write("\n")

	if d.p.Toplevel == nil {
		d.err = errors.New("missing top-level function")
	}
	if d.err == nil {
		d.function(p.Toplevel)
		for _, fn := range p.Functions {
			d.write("\n")
			d.function(fn)
		}
	}

	return d.buf.Bytes(), d.err
}

type dasm struct {
	p   *Program
	buf *bytes.Buffer
	err error
}

func (d *dasm) function(fn *Funcode) {
	if d.err != nil {
		return
	}

	d.writef("function: %s %d %d", fn.Name, fn.MaxStack, fn.NumParams)
	if fn.HasVarargs {
		d.write(" +varargs")
	}
	d.write("\n")

	if len(fn.Locals) > 0 {
		d.write("\tlocals:\n")
		for i, l := range fn.Locals {
			d.writef("\t\t%s\t# %03d\n", l.Name, i)
		}
	}
	if len(fn.Cells) > 0 {
		d.write("\tcells:\n")
		for i, c := range fn.Cells {
			d.writef("\t\t%s\t# %03d\n", fn.Locals[c].Name, i)
		}
	}
	if len(fn.Freevars) > 0 {
		d.write("\tfreevars:\n")
		for i, f := range fn.Freevars {
			d.writef("\t\t%s\t# %03d\n", f.Name, i)
		}
	}

	// decode all instructions to translate addresses to index
	var insns []insn
	addrToIndex := make([]int, len(fn.Code))
	// initialize to -1 to identify invalid jumps
	for i := range addrToIndex {
		addrToIndex[i] = -1
	}
	var addr int
	for addr < len(fn.Code) {
		op := Opcode(fn.Code[addr])
		sz := 1

		var arg uint32
		if op >= OpcodeArgMin {
			v, n := binary.Uvarint(fn.Code[addr+1:])
			if n <= 0 || v > math.MaxUint32 {
				d.err = fmt.Errorf("invalid uvarint argument in function %s code at index %d (%s)", fn.Name, addr, op)
				return
			}
			arg = uint32(v)

			if isJump(op) && n < 4 {
				n = 4
			}
			sz += n
		}

		addrToIndex[addr] = len(insns)
		insns = append(insns, insn{op: op, arg: arg})
		addr += sz
	}

	if len(fn.Defers) > 0 {
		d.write("\tdefers:\n")
		for i, df := range fn.Defers {
			if err := translateDefer(addrToIndex, &df, "defer", fn.Name, i); err != nil { //nolint:gosec
				d.err = err
				return
			}
			d.writef("\t\t%03d %03d %03d\t# %03d\n", df.PC0, df.PC1, df.StartPC, i)
		}
	}

	if len(fn.Catches) > 0 {
		d.write("\tcatches:\n")
		for i, c := range fn.Catches {
			if err := translateDefer(addrToIndex, &c, "catch", fn.Name, i); err != nil { //nolint:gosec
				d.err = err
				return
			}
			d.writef("\t\t%03d %03d %03d\t# %03d\n", c.PC0, c.PC1, c.StartPC, i)
		}
	}

	if len(insns) > 0 {
		d.write("\tcode:\n")
		for i, insn := range insns {
			op, arg := insn.op, insn.arg
			if op >= OpcodeArgMin {
				if isJump(op) {
					if addrToIndex[arg] == -1 {
						d.err = fmt.Errorf("invalid jump address %d in function %s, instruction %d (%s)", arg, fn.Name, i, op)
						return
					}
					arg = uint32(addrToIndex[arg])
				}
				d.writef("\t\t%s %03d\t# %03d\n", op, arg, i)
			} else {
				d.writef("\t\t%s\t# %03d\n", op, i)
			}
		}
	}
}

func translateDefer(addrToIndex []int, defr *Defer, label, fnName string, i int) error {
	if defr.PC0 >= uint32(len(addrToIndex)) {
		return fmt.Errorf("invalid %s.pc0 address in function %s, %s %d", label, fnName, label, i)
	}
	if defr.PC1 >= uint32(len(addrToIndex)) {
		return fmt.Errorf("invalid %s.pc1 address in function %s, %s %d", label, fnName, label, i)
	}
	if defr.StartPC >= uint32(len(addrToIndex)) {
		return fmt.Errorf("invalid %s.startpc address in function %s, %s %d", label, fnName, label, i)
	}

	pc0, pc1, spc := addrToIndex[defr.PC0], addrToIndex[defr.PC1], addrToIndex[defr.StartPC]
	if pc0 < 0 {
		return fmt.Errorf("invalid %s.pc0 address in function %s, %s %d", label, fnName, label, i)
	}
	if pc1 < 0 {
		return fmt.Errorf("invalid %s.pc1 address in function %s, %s %d", label, fnName, label, i)
	}
	if spc < 0 {
		return fmt.Errorf("invalid %s.startpc address in function %s, %s %d", label, fnName, label, i)
	}

	defr.PC0, defr.PC1, defr.StartPC = uint32(pc0), uint32(pc1), uint32(spc)
	return nil
}

func (d *dasm) program() {
	d.write("program:")
	d.write("\n")

	if len(d.p.Loads) > 0 {
		d.write("\tloads:\n")
		for i, l := range d.p.Loads {
			d.writef("\t\t%s\t# %03d\n", l.Name, i)
		}
	}
	if len(d.p.Names) > 0 {
		d.write("\tnames:\n")
		for i, n := range d.p.Names {
			d.writef("\t\t%s\t# %03d\n", n, i)
		}
	}
	if len(d.p.Constants) > 0 {
		d.write("\tconstants:\n")
		for i, c := range d.p.Constants {
			switch c := c.(type) {
			case string:
				d.writef("\t\tstring\t%q\t# %03d\n", c, i)
			case int64:
				d.writef("\t\tint\t%d\t# %03d\n", c, i)
			case float64:
				d.writef("\t\tfloat\t%g\t# %03d\n", c, i)
			default:
				d.err = fmt.Errorf("unsupported constant type: %T", c)
				return
			}
		}
	}
}

func (d *dasm) writef(s string, args ...any) {
	d.write(fmt.Sprintf(s, args...))
}

func (d *dasm) write(s string) {
	if d.err != nil {
		return
	}
	_, d.err = d.buf.WriteString(s)
}
