package machine

import (
	"context"
	"fmt"

	"github.com/mna/nenuphar/lang/compiler"
	"github.com/mna/nenuphar/lang/token"
	"github.com/mna/nenuphar/lang/types"
)

func Run(th *Thread, fn *types.Function, args types.Tuple) (types.Value, error) {
	fcode := fn.Funcode
	if th.DisableRecursion {
		// detect recursion
		for _, fr := range th.callStack[:len(th.callStack)-1] {
			// We look for the same function code, not function value, otherwise the
			// user could defeat the check by writing the Y combinator.
			if frfn, ok := fr.callable.(*types.Function); ok && frfn.Funcode == fcode {
				return nil, fmt.Errorf("function %s called recursively", fn.Name())
			}
		}
	}

	// get the current call frame
	fr := th.callStack[len(th.callStack)-1]

	// create the locals and operand stack
	nlocals := len(fcode.Locals)
	nspace := nlocals + fcode.MaxStack
	space := make([]types.Value, nspace)
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
	// - there is no redefinition of 'inFlightErr'.

	var iterstack []types.Iterator // stack of active iterators

	// Use defer so that application panics can pass through interpreter without
	// leaving thread in a bad state.
	defer func() {
		// ITERPOP the rest of the iterator stack.
		for _, iter := range iterstack {
			iter.Done()
		}
	}()

	var (
		pc          uint32
		result      types.Value
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

		op := Opcode(code[pc])
		pc++
		var arg uint32
		if op >= OpcodeArgMin {
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
		case NOP:
			// nop

		case DUP:
			stack[sp] = stack[sp-1]
			sp++

		case DUP2:
			stack[sp] = stack[sp-2]
			stack[sp+1] = stack[sp-1]
			sp += 2

		case POP:
			sp--

		case EXCH:
			stack[sp-2], stack[sp-1] = stack[sp-1], stack[sp-2]

		case EQL, NEQ, GT, LT, LE, GE:
			op := token.Token(op-EQL) + token.EQL
			y := stack[sp-1]
			x := stack[sp-2]
			sp -= 2
			ok, err := CompareDepth(op, x, y, th.maxCompareDepth)
			if err != nil {
				inFlightErr = err
				break loop
			}
			stack[sp] = types.Bool(ok)
			sp++

		case NIL:
			stack[sp] = types.Nil
			sp++

		case TRUE:
			stack[sp] = types.True
			sp++

		case FALSE:
			stack[sp] = types.False
			sp++

		case JMP:
			if runDefer {
				runDefer = false
				if hasDeferredExecution(int64(fr.pc), int64(arg), fcode.Defers, nil, &pc) {
					deferredStack = append(deferredStack, int64(arg)) // push
					break
				}
			}
			pc = arg

		case NOT:
			v := stack[sp-1]
			switch v := v.(type) {
			case types.Bool:
				stack[sp-1] = !v
			case types.NilType:
				// always false, so !false == true
				stack[sp-1] = types.True
			default:
				// always true, so !true == false
				stack[sp-1] = types.False
			}

		case RETURN:
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

		case MAKEMAP:
			stack[sp] = types.NewMap(int(arg))
			sp++
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
func setArgs(locals []types.Value, fn *types.Function, args types.Tuple) error {

	// Arguments are processed as follows:
	// - positional arguments are bound to locals
	// - surplus positional arguments are bound to the final local if the function
	//   accepts varargs

	// nparams is the number of parameters
	nparams := fn.Funcode.NumParams

	// nullary function?
	if nparams == 0 {
		if nactual := len(args); nactual > 0 {
			return fmt.Errorf("function %s accepts no arguments (%d given)", fn.Name(), nactual)
		}
		return nil
	}

	if fn.Funcode.HasVarargs {
		nparams--
	} else if len(args) > nparams {
		return fmt.Errorf("function %s accepts at most %d arguments (%d given)", fn.Name(), nparams, len(args))
	}

	// bind positional arguments (TODO: should Nil values be already in args, or should it be padded here?)
	for i := 0; i < nparams; i++ {
		locals[i] = args[i]
	}

	// bind surplus positional arguments to *args parameter
	if fn.Funcode.HasVarargs {
		tuple := make(types.Tuple, len(args)-nparams)
		for i := nparams; i < len(args); i++ {
			tuple[i-nparams] = args[i]
		}
		locals[nparams] = tuple
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
