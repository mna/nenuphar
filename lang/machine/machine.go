// Much of the machine package is adapted from the Starlark source code:
// https://github.com/google/starlark-go/tree/ee8ed142361c69d52fe8e9fb5e311d2a0a7c02de
//
// Copyright 2017 The Bazel Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package machine implements the virtual machine that executes the
// bytecode-compiled form of the source code. It also provides the runtime
// representation of the various builtin values.
package machine

import (
	"context"
	"fmt"

	"github.com/mna/nenuphar/lang/compiler"
	"github.com/mna/nenuphar/lang/token"
)

func run(th *Thread, fn *Function, args *Tuple) (Value, error) {
	fcode := fn.Funcode
	if th.DisableRecursion {
		// detect recursion
		for _, fr := range th.callStack[:len(th.callStack)-1] {
			// We look for the same function code, not function value, otherwise the
			// user could defeat the check by writing the Y combinator.
			if frfn, ok := fr.callable.(*Function); ok && frfn.Funcode == fcode {
				return nil, fmt.Errorf("function %s called recursively", fn.Name())
			}
		}
	}

	// get the current call frame
	fr := th.callStack[len(th.callStack)-1]

	// create the locals and operand stack
	nlocals := len(fcode.Locals)
	nspace := nlocals + fcode.MaxStack
	// TODO: experiment with allocating a big slice at startup (configurable)
	// and allocate from it as a big stack when possible, measure gains.
	space := make([]Value, nspace)
	locals := space[:nlocals:nlocals] // local variables, starting with parameters
	stack := space[nlocals:]          // operand stack

	// digest arguments and set parameters
	if err := setArgs(locals, fn, args); err != nil {
		return nil, err
	}

	// create the deferred stack
	// TODO: currently this is naive and just counts the number of
	// defers/catches, but the exact stack size should be known statically.
	var deferredStack []int64
	if n := len(fcode.Defers) + len(fcode.Catches); n > 0 {
		deferredStack = make([]int64, 0, n)
	}

	// Spill indicated locals to cells. Each cell is a separate alloc to avoid
	// spurious liveness.
	for _, index := range fcode.Cells {
		locals[index] = &cell{locals[index]}
	}

	// TODO: add static check that beneath this point
	// - there is exactly one return statement

	// TODO: store static size of iterstack based on loops?
	var iterstack []Iterator // stack of active iterators

	// Use defer so that application panics can pass through interpreter without
	// leaving thread in a bad state.
	defer func() {
		// ITERPOP the rest of the iterator stack.
		for _, iter := range iterstack {
			iter.Done()
		}
	}()

	// NOTE: any opcode that may assign an inFlightErr *must* be compiled
	// with its position stored in compiler/compiler.go (setPos) to result
	// in precise errors.
	var (
		pc          uint32
		result      Value
		runDefer    bool
		inFlightErr error
	)

	sp := 0
	code := fcode.Code
loop:
	for {
		th.steps++
		if th.steps >= th.maxSteps {
			th.ctxCancel()
			// TODO: critical, non-catchable error
			inFlightErr = fmt.Errorf("thread cancelled: %s", context.Cause(th.ctx))
			break loop
		}
		if th.cancelled.Load() {
			// TODO: critical, non-catchable error
			inFlightErr = fmt.Errorf("thread cancelled: %s", context.Cause(th.ctx))
			break loop
		}

		fr.pc = pc

		op := compiler.Opcode(code[pc])
		pc++
		var arg uint32
		if op >= compiler.OpcodeArgMin {
			// TODO(opt): profile this, perhaps compiling big endian would be less
			// work to decode?
			for s := uint(0); ; s += 7 {
				b := code[pc]
				pc++
				arg |= uint32(b&0x7f) << s
				if b < 0x80 {
					break
				}
			}
		}

		switch op {
		case compiler.NOP:
			// nop

		case compiler.DUP:
			stack[sp] = stack[sp-1]
			sp++

		case compiler.DUP2:
			stack[sp] = stack[sp-2]
			stack[sp+1] = stack[sp-1]
			sp += 2

		case compiler.POP:
			sp--

		case compiler.EXCH:
			stack[sp-2], stack[sp-1] = stack[sp-1], stack[sp-2]

		case compiler.EQL, compiler.NEQ, compiler.GT, compiler.LT, compiler.LE, compiler.GE:
			op := token.Token(op-compiler.EQL) + token.EQEQ
			y := stack[sp-1]
			x := stack[sp-2]
			sp -= 2
			ok, err := Compare(op, x, y)
			if err != nil {
				inFlightErr = err
				break loop
			}
			stack[sp] = Bool(ok)
			sp++

		case compiler.PLUS, compiler.MINUS, compiler.STAR, compiler.SLASH,
			compiler.SLASHSLASH, compiler.PERCENT, compiler.CIRCUMFLEX,
			compiler.AMPERSAND, compiler.PIPE, compiler.TILDE,
			compiler.LTLT, compiler.GTGT:

			binop := token.Token(op-compiler.PLUS) + token.PLUS
			y := stack[sp-1]
			x := stack[sp-2]
			sp -= 2
			z, err := Binary(binop, x, y)
			if err != nil {
				inFlightErr = err
				break loop
			}
			stack[sp] = z
			sp++

		case compiler.UPLUS, compiler.UMINUS, compiler.UTILDE, compiler.POUND:
			var unop token.Token
			switch op {
			case compiler.UTILDE:
				// tilde token is out of order
				unop = token.TILDE
			case compiler.POUND:
				// pound token is out of order
				unop = token.POUND
			default:
				unop = token.Token(op-compiler.UPLUS) + token.PLUS
			}
			x := stack[sp-1]
			sp--
			y, err := Unary(unop, x)
			if err != nil {
				inFlightErr = err
				break loop
			}
			stack[sp] = y
			sp++

		case compiler.NIL:
			stack[sp] = Nil
			sp++

		case compiler.TRUE:
			stack[sp] = True
			sp++

		case compiler.FALSE:
			stack[sp] = False
			sp++

		case compiler.JMP:
			if runDefer {
				runDefer = false
				if hasDeferredExecution(int64(fr.pc), int64(arg), fcode.Defers, nil, &pc) {
					deferredStack = append(deferredStack, int64(arg)) // push
					break
				}
			}
			pc = arg

		case compiler.CALL /*, compiler.CALL_VAR*/ :
			//var varArgs Value
			//if op == compiler.CALL_VAR {
			//	varArgs = stack[sp-1]
			//	sp--
			//}

			var positional []Value
			if arg > 0 {
				positional = stack[sp-int(arg) : sp]
				sp -= int(arg)

				// Copy positional arguments into a new array, unless the callee is
				// another Function, in which case it can be trusted not to mutate
				// them.
				if _, ok := stack[sp-1].(*Function); !ok /*|| varArgs != nil*/ {
					positional = append([]Value(nil), positional...)
				}
			}
			//if varArgs != nil {
			//	// TODO: implement vararg parameter passing
			//}

			function := stack[sp-1]
			sp--

			argsTup := NilaryTuple
			if len(positional) > 0 {
				argsTup = NewTuple(positional)
			}
			z, err := Call(th, function, argsTup)
			if err != nil {
				inFlightErr = err
				break loop
			}
			stack[sp] = z
			sp++

		case compiler.ITERPUSH:
			x := stack[sp-1]
			sp--
			iter := Iterate(x)
			if iter == nil {
				inFlightErr = fmt.Errorf("%s value is not iterable", x.Type())
				break loop
			}
			iterstack = append(iterstack, iter)

		case compiler.ITERJMP:
			iter := iterstack[len(iterstack)-1]
			if iter.Next(&stack[sp]) {
				sp++
			} else {
				if runDefer {
					runDefer = false
					if hasDeferredExecution(int64(fr.pc), int64(arg), fcode.Defers, nil, &pc) {
						deferredStack = append(deferredStack, int64(arg)) // push
						break
					}
				}
				pc = arg
			}

		case compiler.ITERPOP:
			n := len(iterstack) - 1
			iterstack[n].Done()
			iterstack = iterstack[:n]

		case compiler.NOT:
			stack[sp-1] = !Truth(stack[sp-1])

		case compiler.RETURN:
			// TODO(mna): if we allow RETURN in a defer, does that clear the
			// inFlightErr? I think we should only allow it in a catch, so that
			// RETURN always clears inFlightErr (and CATCHJMP is not needed when a
			// catch ends in a return).
			result = stack[sp-1]
			sp--
			inFlightErr = nil
			if runDefer {
				runDefer = false
				// a RETURN "to" address is never covered by a deferred block (it jumps
				// outside the function), so run any defers that covers the "from" pc
				// (ignore catch blocks).
				if hasDeferredExecution(int64(fr.pc), -1, fcode.Defers, nil, &pc) {
					// -1 means break loop and return whatever result and inFlightErr are
					// present
					deferredStack = append(deferredStack, -1) // push
					break
				}
			}
			break loop

		case compiler.MAKEMAP:
			stack[sp] = NewMap(int(arg))
			sp++

		//case compiler.UNPACK:
		//	n := int(arg)
		//	iterable := stack[sp-1]
		//	sp--

		//	iter := Iterate(iterable)
		//	if iter == nil {
		//		inFlightErr = fmt.Errorf("%s value is not iterable", iterable.Type())
		//		break loop
		//	}

		//	i := 0
		//	sp += n
		//	for i < n && iter.Next(&stack[sp-1-i]) {
		//		i++
		//	}

		//	// TODO: define UNPACK semantics - fill missing values with null, ignore extra ones?
		//	//var dummy Value
		//	//if iter.Next(&dummy) {
		//	//	// NB: Len may return -1 here in obscure cases.
		//	//	inFlightErr = fmt.Errorf("too many values to unpack (got %d, want %d)", Len(iterable), n)
		//	//	break loop
		//	//}
		//	iter.Done()
		//	//if i < n {
		//	//	inFlightErr = fmt.Errorf("too few values to unpack (got %d, want %d)", i, n)
		//	//	break loop
		//	//}

		case compiler.CJMP:
			if Truth(stack[sp-1]) {
				if runDefer {
					runDefer = false
					if hasDeferredExecution(int64(fr.pc), int64(arg), fcode.Defers, nil, &pc) {
						deferredStack = append(deferredStack, int64(arg)) // push
						break
					}
				}
				pc = arg
			}
			sp--

		case compiler.CONSTANT:
			stack[sp] = fn.Module.Constants[arg]
			sp++

		case compiler.MAKETUPLE:
			n := int(arg)
			elems := make([]Value, n)
			sp -= n
			copy(elems, stack[sp:])
			stack[sp] = NewTuple(elems)
			sp++

		case compiler.MAKEARRAY:
			n := int(arg)
			elems := make([]Value, n)
			sp -= n
			copy(elems, stack[sp:])
			stack[sp] = NewArray(elems)
			sp++

		case compiler.MAKEFUNC:
			funcode := fn.Module.Program.Functions[arg]
			freevars := stack[sp-1].(*Tuple) // ok to panic otherwise, compiler error
			stack[sp-1] = &Function{
				Funcode:  funcode,
				Module:   fn.Module,
				Freevars: freevars,
			}

		case compiler.LOAD:
			m := stack[sp-1]
			sp--

			if th.Load == nil {
				inFlightErr = fmt.Errorf("load not implemented by this application")
				break loop
			}

			s, ok := m.(String)
			if !ok {
				inFlightErr = fmt.Errorf("attempt to load non-string module: %s", m.Type())
				break loop
			}

			v, err := th.Load(th, string(s))
			if err != nil {
				inFlightErr = fmt.Errorf("cannot load %s: %w", s, err)
				break loop
			}
			stack[sp] = v
			sp++

		case compiler.SETINDEX:
			z := stack[sp-1]
			y := stack[sp-2]
			x := stack[sp-3]
			sp -= 3
			if err := setIndex(x, y, z); err != nil {
				inFlightErr = err
				break loop
			}

		case compiler.INDEX:
			y := stack[sp-1]
			x := stack[sp-2]
			sp -= 2
			z, err := getIndex(x, y)
			if err != nil {
				inFlightErr = err
				break loop
			}
			stack[sp] = z
			sp++

		case compiler.ATTR:
			x := stack[sp-1]
			sp--
			name := fn.Module.Program.Names[arg]
			y, err := getAttr(x, name)
			if err != nil {
				inFlightErr = err
				break loop
			}
			stack[sp] = y
			sp++

		case compiler.SETFIELD:
			y := stack[sp-1]
			x := stack[sp-2]
			sp -= 2
			name := fn.Module.Program.Names[arg]
			if err := setField(x, name, y); err != nil {
				inFlightErr = err
				break loop
			}

		case compiler.SETMAP:
			m := stack[sp-3].(*Map) // ok to panic otherwise, compiler error (this is emitted only in map literals)
			k := stack[sp-2]
			v := stack[sp-1]
			sp -= 3
			if err := m.SetKey(k, v); err != nil {
				inFlightErr = err
				break loop
			}

		case compiler.SETLOCAL:
			locals[arg] = stack[sp-1]
			sp--

		case compiler.SETLOCALCELL:
			locals[arg].(*cell).v = stack[sp-1] // ok to panic otherwise, compiler error
			sp--

		case compiler.LOCAL:
			x := locals[arg]
			if x == nil {
				inFlightErr = fmt.Errorf("local variable %s referenced before assignment", fcode.Locals[arg].Name)
				break loop
			}
			stack[sp] = x
			sp++

		case compiler.FREE:
			stack[sp] = fn.Freevars.Index(int(arg))
			sp++

		case compiler.LOCALCELL:
			v := locals[arg].(*cell).v // ok to panic otherwise, compiler error
			if v == nil {
				inFlightErr = fmt.Errorf("local variable %s referenced before assignment", fcode.Locals[arg].Name)
				break loop
			}
			stack[sp] = v
			sp++

		case compiler.FREECELL:
			v := fn.Freevars.Index(int(arg)).(*cell).v // ok to panic otherwise, compiler error
			if v == nil {
				inFlightErr = fmt.Errorf("local variable %s referenced before assignment", fcode.Freevars[arg].Name)
				break loop
			}
			stack[sp] = v
			sp++

		case compiler.PREDECLARED:
			name := fn.Module.Program.Names[arg]
			x := th.Predeclared[name]
			if x == nil {
				inFlightErr = fmt.Errorf("internal error: predeclared variable %s is uninitialized", name) // TODO: does not exist?
				break loop
			}
			stack[sp] = x
			sp++

		case compiler.UNIVERSAL:
			stack[sp] = Universe[fn.Module.Program.Names[arg]] // TODO: check nil and fail if does not exist? panic, compiler error?
			sp++

		case compiler.RUNDEFER:
			// TODO(opt): for defers, it is known statically what defer should run,
			// so this opcode could encode as argument the index of the defer to run,
			// and then DEFEREXIT could do the same for the next one (if there are
			// many to run). Hmm or actually for DEFEREXIT it is not known
			// statically, as a defer can be triggered via multiple RUNDEFER. But at
			// least for RUNDEFER it is known.
			runDefer = true

		case compiler.DEFEREXIT:
			// read target address but do not pop it yet, depends if there's more
			// deferred execution to run.
			returnTo := deferredStack[len(deferredStack)-1] // peek

			// if there's an in-flight error, the next deferred execution could be a
			// catch (e.g. a defer could've been the first deferred execution when it
			// was raised, and a catch is still possible). Otherwise, do not consider
			// them.
			var catch []compiler.Defer
			if inFlightErr != nil {
				catch = fcode.Catches
			}
			if hasDeferredExecution(int64(fr.pc), returnTo, fcode.Defers, catch, &pc) {
				break
			}

			deferredStack = deferredStack[:len(deferredStack)-1] // pop
			if returnTo < 0 {
				break loop
			}
			pc = uint32(returnTo)

		case compiler.CATCHJMP:
			// this is the normal exit of a catch block, so it clears the inFlightErr
			// TODO: put that in the frame so the "error" built-in has access to it?
			inFlightErr = nil

			// special-case: if jump address is 0 - which is impossible for a
			// CATCHJMP because it always jumps forward to after the parent block -,
			// treat it as -1 and set the return value to `none` (i.e. it is
			// equivalent to a top-level catch in a function, it covers the whole
			// function and on error acts as if there was no explicit RETURN, which
			// means an implicit 'return nil' in high-level language syntax.
			returnTo := int64(arg)
			if arg == 0 {
				result = Nil
				returnTo = -1
			}
			if hasDeferredExecution(int64(fr.pc), returnTo, fcode.Defers, nil, &pc) {
				deferredStack = append(deferredStack, returnTo) // push
				break
			}
			if returnTo < 0 {
				break loop
			}
			pc = arg

		default:
			panic(fmt.Sprintf("unimplemented: %s", op))
		}
	}

	if inFlightErr != nil {
		if hasDeferredExecution(int64(fr.pc), -1, fcode.Defers, fcode.Catches, &pc) {
			// by default, pending action is to exit the function
			deferredStack = append(deferredStack, -1) // push
			goto loop
		}
	}

	return result, inFlightErr
}

// setArgs sets the values of the formal parameters of function fn in
// based on the actual parameter values in args and kwargs.
func setArgs(locals []Value, fn *Function, args *Tuple) error {

	// Arguments are processed as follows:
	// - positional arguments are bound to locals
	// - surplus positional arguments are bound to the final local if the function
	//   accepts varargs

	// nparams is the number of parameters
	nparams := fn.Funcode.NumParams
	nargs := args.Len()

	// nullary function?
	if nparams == 0 {
		if nargs > 0 {
			return fmt.Errorf("function %s accepts no arguments (%d given)", fn.Name(), nargs)
		}
		return nil
	}

	if fn.Funcode.HasVarArg {
		nparams--
	} else if nargs > nparams {
		return fmt.Errorf("function %s accepts at most %d arguments (%d given)", fn.Name(), nparams, nargs)
	}

	// bind positional arguments (TODO: should Nil values be already in args, or should it be padded here?)
	for i := 0; i < nparams; i++ {
		locals[i] = args.Index(i)
	}

	// bind surplus positional arguments to *args parameter
	if fn.Funcode.HasVarArg {
		elems := make([]Value, nargs-nparams)
		for i := nparams; i < nargs; i++ {
			elems[i-nparams] = args.Index(i)
		}
		locals[nparams] = NewTuple(elems)
	}
	return nil
}

// TODO(opt): check if this would benefit from being done inline, and if
// something like an interval tree would be faster than looping through all
// defers/catches (I suspect looping is faster when n is small and would
// generally be very small, i.e. < 10 and probably even < 5).
func hasDeferredExecution(from, to int64, defr, catch []compiler.Defer, pc *uint32) bool {
	target := -1
	for _, d := range defr {
		if d.Covers(from) && !d.Covers(to) {
			if int(d.StartPC) > target {
				target = int(d.StartPC)
			}
		}
	}
	for _, d := range catch {
		if d.Covers(from) && !d.Covers(to) {
			if int(d.StartPC) > target {
				target = int(d.StartPC)
			}
		}
	}
	if target >= 0 {
		*pc = uint32(target)
		return true
	}
	return false
}
