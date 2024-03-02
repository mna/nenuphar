package types

import (
	"github.com/mna/nenuphar-wip/syntax"
)

// A Function is a function defined by a function statement or expression. The
// initialization behavior of a module is also represented by a (top-level)
// Function.
type Function struct {
	Funcode  *compiler.Funcode
	module   *module
	defaults Tuple // TODO: support default values?
	freevars Tuple
}

// A module is the dynamic counterpart to a compiler.Program, which is the unit
// of compilation. All functions in the same program share a module.
type module struct {
	program     *compiler.Program
	predeclared StringDict // TODO: here or just provided to a Thread (e.g. like Env for Lua)?
	globals     []Value    // TODO: no globals, only locals to the top-level function?
	constants   []Value
}

func (fn *Function) Name() string              { return fn.funcode.Name } // "lambda" for anonymous functions
func (fn *Function) Freeze()                   { fn.defaults.Freeze(); fn.freevars.Freeze() }
func (fn *Function) String() string            { return "<function " + fn.Name() + ">" }
func (fn *Function) Type() string              { return "function" }
func (fn *Function) Truth() Bool               { return true }
func (fn *Function) Position() syntax.Position { return fn.funcode.Pos }
func (fn *Function) NumParams() int            { return fn.funcode.NumParams }
func (fn *Function) NumKwonlyParams() int      { return fn.funcode.NumKwonlyParams }

// Param returns the name and position of the ith parameter, where 0 <= i <
// NumParams(). The *args and **kwargs parameters are at the end even if there
// were optional parameters after *args.
func (fn *Function) Param(i int) (string, syntax.Position) {
	if i >= fn.NumParams() {
		panic(i)
	}
	id := fn.funcode.Locals[i]
	return id.Name, id.Pos
}

// ParamDefault returns the default value of the specified parameter (0 <= i <
// NumParams()), or nil if the parameter is not optional.
func (fn *Function) ParamDefault(i int) Value {
	if i < 0 || i >= fn.NumParams() {
		panic(i)
	}

	// fn.defaults omits all required params up to the first optional param. It
	// also does not include *args or **kwargs at the end.
	firstOptIdx := fn.NumParams() - len(fn.defaults)
	if fn.HasVarargs() {
		firstOptIdx--
	}
	if fn.HasKwargs() {
		firstOptIdx--
	}
	if i < firstOptIdx || i >= firstOptIdx+len(fn.defaults) {
		return nil
	}

	dflt := fn.defaults[i-firstOptIdx]
	if _, ok := dflt.(mandatory); ok {
		return nil
	}
	return dflt
}

func (fn *Function) HasVarargs() bool { return fn.funcode.HasVarargs }
func (fn *Function) HasKwargs() bool  { return fn.funcode.HasKwargs }
