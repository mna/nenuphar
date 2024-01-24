package types

// Bool is the type of boolean values.
type Bool bool

const (
	False Bool = false
	True  Bool = true
)

// Bool is a Value.
var _ Value = True

func (b Bool) String() string {
	if b {
		return "true"
	}
	return "false"
}

func (b Bool) Type() string { return "bool" }
func (b Bool) Freeze()      {} // immutable
func (b Bool) Truth() Bool  { return b }

func (b Bool) Cmp(y Value, depth int) (int, error) {
	b2 := y.(Bool)
	return b2i(bool(b)) - b2i(bool(b2)), nil
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}
