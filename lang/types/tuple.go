package types

// A Tuple represents an immutable list of values.
type Tuple []Value

// TODO: or just use array that can be made immutable?

var (
	_ Value = Tuple(nil)
)

func (t Tuple) String() string { return "TODO(tuple)" }
func (t Tuple) Type() string   { return "tuple" }
