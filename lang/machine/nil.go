package machine

// NilType is the type of nil. Its only legal value is Nil. (We represent it as
// a number, not struct{}, so that Nil may be constant.)
type NilType byte

const Nil = NilType(0)

// Nil is a Value.
var _ Value = Nil

func (NilType) String() string { return "nil" }
func (NilType) Type() string   { return "nil" }
