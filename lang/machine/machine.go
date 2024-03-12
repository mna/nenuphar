package machine

import (
	"context"
	"fmt"

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

		case UPLUS, UMINUS, TILDE:
			var unop token.Token
			if op == TILDE {
				unop = token.TILDE
			} else {
				unop = token.Token(op-UPLUS) + token.PLUS
			}
			x := stack[sp-1]
			y, err := Unary(unop, x)
			if err != nil {
				inFlightErr = err
				break loop
			}
			stack[sp-1] = y

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

		case CALL, CALL_VAR, CALL_KW, CALL_VAR_KW:
			var kwargs types.Value
			if op == CALL_KW || op == CALL_VAR_KW {
				kwargs = stack[sp-1]
				sp--
			}

			var args types.Value
			if op == CALL_VAR || op == CALL_VAR_KW {
				args = stack[sp-1]
				sp--
			}

			// named args (pairs)
			var kvpairs []types.Tuple
			if nkvpairs := int(arg & 0xff); nkvpairs > 0 {
				kvpairs = make([]types.Tuple, 0, nkvpairs)
				kvpairsAlloc := make(types.Tuple, 2*nkvpairs) // allocate a single backing array
				sp -= 2 * nkvpairs
				for i := 0; i < nkvpairs; i++ {
					pair := kvpairsAlloc[:2:2]
					kvpairsAlloc = kvpairsAlloc[2:]
					pair[0] = stack[sp+2*i]   // name
					pair[1] = stack[sp+2*i+1] // value
					kvpairs = append(kvpairs, pair)
				}
			}
			if kwargs != nil {
				// Add key/value items from **kwargs dictionary.
				dict, ok := kwargs.(types.IterableMapping)
				if !ok {
					inFlightErr = fmt.Errorf("argument after ** must be a mapping, not %s", kwargs.Type())
					break loop
				}
				items := dict.Items()
				for _, item := range items {
					if _, ok := item[0].(types.String); !ok {
						inFlightErr = fmt.Errorf("keywords must be strings, not %s", item[0].Type())
						break loop
					}
				}
				if len(kvpairs) == 0 {
					kvpairs = items
				} else {
					kvpairs = append(kvpairs, items...)
				}
			}

			// positional args
			var positional types.Tuple
			if npos := int(arg >> 8); npos > 0 {
				positional = stack[sp-npos : sp]
				sp -= npos

				// Copy positional arguments into a new array,
				// unless the callee is another Starlark function,
				// in which case it can be trusted not to mutate them.
				if _, ok := stack[sp-1].(*types.Function); !ok || args != nil {
					positional = append(types.Tuple(nil), positional...)
				}
			}
			if args != nil {
				// Add elements from *args sequence.
				iter := Iterate(args)
				if iter == nil {
					inFlightErr = fmt.Errorf("argument after * must be iterable, not %s", args.Type())
					break loop
				}
				var elem types.Value
				for iter.Next(&elem) {
					positional = append(positional, elem)
				}
				iter.Done()
			}

			function := stack[sp-1]

			z, err := Call(th, function, positional, kvpairs)
			if err != nil {
				inFlightErr = err
				break loop
			}
			stack[sp-1] = z

		case ITERPUSH:
			x := stack[sp-1]
			sp--
			iter := Iterate(x)
			if iter == nil {
				inFlightErr = fmt.Errorf("%s value is not iterable", x.Type())
				break loop
			}
			iterstack = append(iterstack, iter)

		case ITERJMP:
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

		case ITERPOP:
			n := len(iterstack) - 1
			iterstack[n].Done()
			iterstack = iterstack[:n]

		case NOT:
			stack[sp-1] = !stack[sp-1].Truth()

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

		case SETINDEX:
			z := stack[sp-1]
			y := stack[sp-2]
			x := stack[sp-3]
			sp -= 3
			inFlightErr = setIndex(x, y, z)
			if inFlightErr != nil {
				break loop
			}

		case INDEX:
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

		case ATTR:
			x := stack[sp-1]
			name := fcode.Prog.Names[arg]
			y, err := getAttr(x, name)
			if err != nil {
				inFlightErr = err
				break loop
			}
			stack[sp-1] = y

		case SETFIELD:
			y := stack[sp-1]
			x := stack[sp-2]
			sp -= 2
			name := fcode.Prog.Names[arg]
			if err := setField(x, name, y); err != nil {
				inFlightErr = err
				break loop
			}

		case MAKEMAP:
			stack[sp] = types.NewMap(0)
			sp++

		case SETMAP:
			m := stack[sp-3].(*types.Map) // TODO: shouldn't that generate a catchable runtime error?
			k := stack[sp-2]
			v := stack[sp-1]
			sp -= 3
			if err := m.SetKey(k, v); err != nil {
				inFlightErr = err
				break loop
			}

		case APPEND:
			elem := stack[sp-1]
			list := stack[sp-2].(*types.Array) // TODO: shouldn't that generate a catchable runtime error?
			sp -= 2
			list.elems = append(list.elems, elem)
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
