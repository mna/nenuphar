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

// Call calls the function or Callable value fn with the specified positional
// and keyword arguments.
func Call(thread *Thread, fn types.Value, args types.Tuple, kwargs []types.Tuple) (types.Value, error) {
	c, ok := fn.(types.Callable)
	if !ok {
		return nil, fmt.Errorf("invalid call of non-function (%s)", fn.Type())
	}

	// Allocate and push a new frame.
	var fr *Frame
	// Optimization: use slack portion of thread.stack
	// slice as a freelist of empty frames.
	if n := len(thread.stack); n < cap(thread.stack) {
		fr = thread.stack[n : n+1][0]
	}
	if fr == nil {
		fr = new(Frame)
	}

	if thread.stack == nil {
		// one-time initialization of thread
		if thread.maxSteps == 0 {
			thread.maxSteps-- // (MaxUint64)
		}
	}

	thread.stack = append(thread.stack, fr) // push

	fr.callable = c

	thread.beginProfSpan()

	// Use defer to ensure that panics from built-ins
	// pass through the interpreter without leaving
	// it in a bad state.
	defer func() {
		thread.endProfSpan()

		// clear out any references
		// TODO(adonovan): opt: zero fr.Locals and
		// reuse it if it is large enough.
		*fr = Frame{}

		thread.stack = thread.stack[:len(thread.stack)-1] // pop
	}()

	result, err := c.CallInternal(thread, args, kwargs)

	// Sanity check: nil is not a valid Starlark value.
	if result == nil && err == nil {
		err = fmt.Errorf("internal error: nil (not None) returned from %s", fn)
	}

	// Always return an EvalError with an accurate frame.
	if err != nil {
		if _, ok := err.(*EvalError); !ok {
			err = thread.evalError(err)
		}
	}

	return result, err
}
