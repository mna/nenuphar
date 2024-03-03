package machine

import (
	"context"
	"io"
	"sync/atomic"

	"github.com/mna/nenuphar/lang/types"
)

type Thread struct {
	// Name is an optional name that describes the thread, mostly for debugging.
	Name string

	// Stdout, Stderr and Stdin are the standard I/O abstractions for the thread.
	// If nil, os.Stdout, os.Stderr and os.Stdin are used, respectively.
	Stdout io.Writer
	Stderr io.Writer
	Stdin  io.Reader

	// MaxSteps is the maximum number of "steps", a deliberately unspecified
	// measure of machine execution time, before the thread is cancelled. A value
	// <= 0 means no limit.
	MaxSteps int

	// DisableRecursion prevents recursive execution of functions when set to
	// true. It incurs a small performance cost for the runtime verification on
	// each function call but can be a useful safety check when executing
	// untrusted code. If a recursive call is detected when set to true, the
	// thread is cancelled.
	DisableRecursion bool

	// MaxCallStackDepth limits the number of nested function calls. If the limit
	// is reached, the thread is cancelled. A value <= 0 means no limit.
	MaxCallStackDepth int

	// MaxCompareDepth limits the number of nested comparison depth for compound
	// types to prevent comparing cyclic values.
	MaxCompareDepth int

	// Load is an optional function value to call to load modules (called by the
	// LOAD opcode).
	Load func(*Thread, string) (types.Value, error)

	ctx       context.Context
	ctxCancel func()
	callStack []*Frame
	cancelled atomic.Bool

	steps, maxSteps uint64
	maxCompareDepth uint64
}

func (th *Thread) init() {
	// one-time initialization of thread
	if th.MaxSteps <= 0 {
		th.maxSteps-- // (MaxUint64)
	} else {
		th.maxSteps = uint64(th.MaxSteps)
	}
	if th.MaxCompareDepth <= 0 {
		th.maxCompareDepth-- // (MaxUint64)
	} else {
		th.maxCompareDepth = uint64(th.MaxCompareDepth)
	}
	if th.ctx == nil {
		th.ctx = context.Background()
		th.ctxCancel = func() {}
	} else {
		go func() {
			<-th.ctx.Done()
			th.cancelled.Store(true)
		}()
	}
}
