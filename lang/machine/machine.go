package machine

import (
	"context"
	"fmt"

	"github.com/mna/nenuphar/lang/token"
	"github.com/mna/nenuphar/lang/types"
)

func Run(th *Thread, fn *types.Function, args types.Tuple, kwargs []types.Tuple) (types.Value, error) {
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

	// Digest arguments and set parameters.
	if err := setArgs(locals, fn, args, kwargs); err != nil {
		return nil, th.evalError(err)
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
			th.Cancel("too many steps")
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

		case PLUS, MINUS, STAR, SLASH, SLASHSLASH, PERCENT, AMPERSAND,
			PIPE, CIRCUMFLEX, LTLT, GTGT, IN:

			binop := token.Token(op-PLUS) + token.PLUS
			if op == IN {
				binop = token.IN // IN token is out of order
			}
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

// TODO(opt): check if this would benefit from being done inline, and if
// something like an interval tree would be faster than looping through all
// defers/catches (I suspect looping is faster when n is small and would
// generally be very small, i.e. < 10 and probably even < 5).
func hasDeferredExecution(from, to int64, defr, catch []compile.Defer, pc *uint32) bool {
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
