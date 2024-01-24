package types

import (
	"context"
	"io"
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
	// measure of machine execution time, before the thread is cancelled.
	MaxSteps int

	Load func(*Thread, string) (Value, error)

	ctx       context.Context
	ctxCancel func()
}
