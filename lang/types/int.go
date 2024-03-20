package types

import (
	"strconv"
)

// Int is the type of a integer value. Iteration on an integer yields all
// integers from 0 to the integer value (not included).
type Int int64

var (
	_ Value    = Int(0)
	_ Ordered  = Int(0)
	_ Iterable = Int(0)
)

func (i Int) String() string {
	return strconv.FormatInt(int64(i), 10)
}

func (i Int) Type() string { return "int" }

func (i Int) Cmp(v Value) (int, error) {
	j := v.(Int)
	if i > j {
		return +1, nil
	} else if i < j {
		return -1, nil
	}
	return 0, nil
}

func (i Int) Iterate() Iterator {
	return &intIterator{n: int64(i)}
}

type intIterator struct {
	i int64
	n int64
}

func (it *intIterator) Next(p *Value) bool {
	if it.i < it.n {
		*p = Int(it.i)
		it.i++
		return true
	}
	return false
}

func (it *intIterator) Done() {}
