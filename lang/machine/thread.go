package machine

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync/atomic"

	"github.com/mna/nenuphar/lang/compiler"
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
	// types to prevent comparing cyclic values. A value <= 0 means no limit.
	MaxCompareDepth int

	// Load is an optional function value to call to load modules (called by the
	// LOAD opcode).
	Load func(*Thread, string) (types.Value, error)

	// Predeclared is the set of predeclared identifiers and their assigned
	// values. Predeclared identifiers are like the Universe identifiers in that
	// they are available to all modules automatically and they cannot be
	// assigned to.
	Predeclared map[string]types.Value

	ctx       context.Context
	ctxCancel func()
	callStack []*Frame
	cancelled atomic.Bool

	steps, maxSteps uint64
	maxCompareDepth uint64

	stdout io.Writer
	stderr io.Writer
	stdin  io.Reader
}

func (th *Thread) RunProgram(ctx context.Context, p *compiler.Program) (types.Value, error) {
	// TODO: would it be acceptable to run more than one program on a thread?
	if th.ctx != nil {
		return nil, fmt.Errorf("thread %s is already executing a program", th.Name)
	}

	ctx, cancel := context.WithCancel(ctx)
	th.ctx = ctx
	th.ctxCancel = cancel
	topfn := makeToplevelFunction(p)
	return Call(th, topfn, nil)
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
	if th.Stdout != nil {
		th.stdout = th.Stdout
	} else {
		th.stdout = os.Stdout
	}
	if th.Stderr != nil {
		th.stderr = th.Stderr
	} else {
		th.stderr = os.Stderr
	}
	if th.Stdin != nil {
		th.stdin = th.Stdin
	} else {
		th.stdin = os.Stdin
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

func makeToplevelFunction(p *compiler.Program) *types.Function {
	// create the value denoted by each program constant
	constants := make([]types.Value, len(p.Constants))
	for i, c := range p.Constants {
		var v types.Value
		switch c := c.(type) {
		case int64:
			v = types.Int(c)
		case string:
			v = types.String(c)
		case float64:
			v = types.Float(c)
		default:
			panic(fmt.Sprintf("unexpected constant %T: %[1]v", c))
		}
		constants[i] = v
	}

	return &types.Function{
		Funcode: p.Toplevel,
		Module: &types.Module{
			Program:   p,
			Constants: constants,
		},
	}
}
