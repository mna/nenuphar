package types

// Bool is the type of boolean values.
type Bool bool

const (
	False Bool = false
	True  Bool = true
)

// Bool is an ordered value.
var (
	_ Value   = True
	_ Ordered = True
)

func (b Bool) String() string {
	if b {
		return "true"
	}
	return "false"
}

func (b Bool) Type() string { return "bool" }

func (b Bool) Cmp(y Value) (int, error) {
	b2 := y.(Bool)
	return b2i(bool(b)) - b2i(bool(b2)), nil
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}
