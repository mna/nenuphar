package machine

import "fmt"

// Some machine opcodes are more complex and/or need to be exposed via a
// low-level interface to be available for higher-level APIs. Those functions
// belong in this file.

// A Callable value f may be the operand of a function call, f(x). Clients
// should use the Call function, never the CallInternal method.
type Callable interface {
	types.Value
	Name() string
	CallInternal(thread *Thread, args types.Tuple, kwargs []types.Tuple) (types.Value, error)
}

// Call calls the function or Callable value v with the specified positional
// and keyword arguments.
func Call(th *Thread, v types.Value, args types.Tuple, kwargs []types.Tuple) (types.Value, error) {
	var fn *types.Function
	var cb types.Callable

	switch v := v.(type) {
	case *types.Function:
		fn = v
	case types.Callable:
		cb = v
	default:
		return nil, fmt.Errorf("invalid call of non-callable (%s)", fn.Type())
	}

	// Allocate and push a new frame. As an optimization, use slack portion of
	// thread.callStack slice as a freelist of empty frames.
	var fr *Frame
	if n := len(th.callStack); n < cap(th.callStack) {
		fr = th.callStack[n : n+1][0]
	}
	if fr == nil {
		fr = new(Frame)
	}

	if th.callStack == nil {
		th.init()
	}

	th.callStack = append(th.callStack, fr) // push

	// Use defer to ensure that panics from built-ins pass through the
	// interpreter without leaving it in a bad state.
	defer func() {
		// clear out any references
		// TODO(adonovan): opt: zero fr.Locals and reuse it if it is large enough.
		*fr = Frame{}

		th.callStack = th.callStack[:len(th.callStack)-1] // pop
	}()

	var result types.Value
	var err error

	if fn != nil {
		fr.callable = fn
		result, err = Run(th, fn)
	} else {
		fr.callable = cb
		result, err = cb.CallInternal(th, args, kwargs)
	}

	// Sanity check: nil is not a valid value.
	if result == nil && err == nil {
		err = fmt.Errorf("internal error: nil (not None) returned from %s", fn)
	}

	// Always return an EvalError with an accurate frame.
	if err != nil {
		if _, ok := err.(*EvalError); !ok {
			err = th.evalError(err)
		}
	}

	return result, err
}
