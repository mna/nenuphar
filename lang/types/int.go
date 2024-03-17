package types

import (
	"strconv"
)

// Int is the type of a integer value.
type Int int64

var (
	_ Value   = Int(0)
	_ Ordered = Int(0)
)

func (i Int) String() string {
	return strconv.FormatInt(int64(i), 10)
}

func (i Int) Type() string { return "int" }

// Cmp implements comparison of two Int values.
func (i Int) Cmp(v Value) (int, error) {
	j := v.(Int)
	if i > j {
		return +1, nil
	} else if i < j {
		return -1, nil
	}
	return 0, nil
}
