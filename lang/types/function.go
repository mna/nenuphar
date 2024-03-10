package types

// A Function is a function defined by a function statement or expression. The
// initialization behavior of a module is also represented by a (top-level)
// Function.
type Function struct {
	Funcode  *compiler.Funcode
	module   *module
	freevars Tuple
}

var (
	_ Value = (*Function)(nil)
)

// A module is the dynamic counterpart to a compiler.Program, which is the unit
// of compilation. All functions in the same program share a module.
type module struct {
	program     *compiler.Program
	predeclared StringDict // TODO: here or just provided to a Thread (e.g. like Env for Lua)?
	globals     []Value    // TODO: no globals, only locals to the top-level function?
	constants   []Value
}

func (fn *Function) String() string { return "<function " + fn.Name() + ">" }
func (fn *Function) Type() string   { return "function" }
