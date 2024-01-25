package types

import (
	"fmt"

	"github.com/mna/nenuphar-wip/syntax"
)

// An *Array represents a list of values.
type Array struct {
	elems     []Value
	frozen    bool
	itercount uint32 // number of active iterators (ignored if frozen)
}

// NewList returns a list containing the specified elements.
// Callers should not subsequently modify elems.
func NewList(elems []Value) *List { return &List{elems: elems} }

func (l *List) Freeze() {
	if !l.frozen {
		l.frozen = true
		for _, elem := range l.elems {
			elem.Freeze()
		}
	}
}

// checkMutable reports an error if the list should not be mutated.
// verb+" list" should describe the operation.
func (l *List) checkMutable(verb string) error {
	if l.frozen {
		return fmt.Errorf("cannot %s frozen list", verb)
	}
	if l.itercount > 0 {
		return fmt.Errorf("cannot %s list during iteration", verb)
	}
	return nil
}

func (l *List) String() string        { return toString(l) }
func (l *List) Type() string          { return "list" }
func (l *List) Hash() (uint32, error) { return 0, fmt.Errorf("unhashable type: list") }
func (l *List) Truth() Bool           { return l.Len() > 0 }
func (l *List) Len() int              { return len(l.elems) }
func (l *List) Index(i int) Value     { return l.elems[i] }

func (l *List) Slice(start, end, step int) Value {
	if step == 1 {
		elems := append([]Value{}, l.elems[start:end]...)
		return NewList(elems)
	}

	sign := signum(step)
	var list []Value
	for i := start; signum(end-i) == sign; i += step {
		list = append(list, l.elems[i])
	}
	return NewList(list)
}

func (l *List) Attr(name string) (Value, error) { return builtinAttr(l, name, listMethods) }
func (l *List) AttrNames() []string             { return builtinAttrNames(listMethods) }

func (l *List) Iterate() Iterator {
	if !l.frozen {
		l.itercount++
	}
	return &listIterator{l: l}
}

func (l *List) CompareSameType(op syntax.Token, y_ Value, depth int) (bool, error) {
	y := y_.(*List)
	// It's tempting to check x == y as an optimization here,
	// but wrong because a list containing NaN is not equal to itself.
	return sliceCompare(op, l.elems, y.elems, depth)
}

func sliceCompare(op syntax.Token, x, y []Value, depth int) (bool, error) {
	// Fast path: check length.
	if len(x) != len(y) && (op == syntax.EQL || op == syntax.NEQ) {
		return op == syntax.NEQ, nil
	}

	// Find first element that is not equal in both lists.
	for i := 0; i < len(x) && i < len(y); i++ {
		if eq, err := EqualDepth(x[i], y[i], depth-1); err != nil {
			return false, err
		} else if !eq {
			switch op {
			case syntax.EQL:
				return false, nil
			case syntax.NEQ:
				return true, nil
			default:
				return CompareDepth(op, x[i], y[i], depth-1)
			}
		}
	}

	return threeway(op, len(x)-len(y)), nil
}

type listIterator struct {
	l *List
	i int
}

func (it *listIterator) Next(p *Value) bool {
	if it.i < it.l.Len() {
		*p = it.l.elems[it.i]
		it.i++
		return true
	}
	return false
}

func (it *listIterator) Done() {
	if !it.l.frozen {
		it.l.itercount--
	}
}

func (l *List) SetIndex(i int, v Value) error {
	if err := l.checkMutable("assign to element of"); err != nil {
		return err
	}
	l.elems[i] = v
	return nil
}

func (l *List) Append(v Value) error {
	if err := l.checkMutable("append to"); err != nil {
		return err
	}
	l.elems = append(l.elems, v)
	return nil
}

func (l *List) Clear() error {
	if err := l.checkMutable("clear"); err != nil {
		return err
	}
	for i := range l.elems {
		l.elems[i] = nil // aid GC
	}
	l.elems = l.elems[:0]
	return nil
}
