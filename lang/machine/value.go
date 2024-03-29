package machine

import "github.com/mna/nenuphar/lang/token"

// Value is the interface implemented by any value manipulated by the machine.
type Value interface {
	// String returns the string representation of the value.
	String() string

	// Type returns a short string describing the value's type.
	Type() string
}

// A Callable value f may be the operand of a function call, f(x). Clients
// should use the Call function, never the CallInternal method.
type Callable interface {
	Value
	Name() string
	CallInternal(thread *Thread, args *Tuple) (Value, error)
}

// An Ordered type is a type whose values are ordered: if x and y are of the
// same Ordered type, then x must be less than y, greater than y, or equal to
// y.
type Ordered interface {
	Value

	// Cmp compares two values x and y of the same ordered type. It returns
	// negative if x < y, positive if x > y, and zero if the values are equal.
	// Client code should not call this method. Instead, use the standalone
	// Compare function, which is defined for all pairs of operands.
	Cmp(y Value) (int, error)
}

// A HasEqual type is a type which defines a custom equality logic for its
// values. An Ordered type should not implement HasEqual, but if values of a
// type are not ordered but should not use identity equality, then it should
// implement HasEqual.
type HasEqual interface {
	Value
	// Equals returns true if the receiver value is considered equal to y. Client
	// code should not call this method. Instead, use the standalone Compare
	// function, which is defined for all pairs of operands.
	Equals(y Value) (bool, error)
}

// An Iterable abstracts a sequence of values. An iterable value may be
// iterated over. Unlike a Sequence, the length of an Iterable is not
// necessarily known in advance of iteration.
type Iterable interface {
	Value
	// Iterate returns an Iterator. It must be followed by call to Iterator.Done.
	Iterate() Iterator
}

// A Sequence is a sequence of values of known length.
type Sequence interface {
	Iterable
	// Len returns the number of elements in the sequence.
	Len() int
}

// An Indexable is a sequence of known length that supports efficient random
// access. It is not necessarily iterable.
type Indexable interface {
	Value
	// Index returns the value at the specified index, which must satisfy 0 <= i
	// < Len().
	Index(i int) Value
	// Len returns the number of elements in the sequence.
	Len() int
}

// A HasSetIndex is an Indexable value whose elements may be assigned (x[i] =
// y). The implementation should not add Len to a negative index as the
// evaluator does this before the call.
type HasSetIndex interface {
	Indexable
	SetIndex(index int, v Value) error
}

// An Iterator provides a sequence of values to the caller. The caller must
// call Done when the iterator is no longer needed. Operations that modify a
// sequence will fail if it has active iterators.
//
// Example usage:
//
//	iter := iterable.Iterator()
//	defer iter.Done()
//	var x Value
//	for iter.Next(&x) {
//		...
//	}
type Iterator interface {
	// If the iterator is exhausted, Next returns false. Otherwise it sets *p to
	// the current element of the sequence, advances the iterator, and returns
	// true.
	Next(p *Value) bool
	// Done must be called on the Iterator once it is no longer needed.
	Done()
}

// A Mapping is a mapping from keys to values, such as a map.
type Mapping interface {
	Value
	// Get returns the value corresponding to the specified key, or !found if the
	// mapping does not contain the key. TODO: revisit: Get also defines the
	// behavior of "v in mapping". The 'in' operator reports the 'found'
	// component, ignoring errors.
	Get(Value) (v Value, found bool, err error)
}

// A HasSetKey supports map update using x[k]=v syntax.
type HasSetKey interface {
	Mapping
	SetKey(k, v Value) error
}

// A HasBinary value may be used as either operand of the binary operators. The
// Side argument indicates whether the receiver is the left or right operand.
// An implementation may decline to handle an operation by returning (nil,
// nil). For this reason, clients should always call the standalone Binary API
// function rather than calling the method directly.
type HasBinary interface {
	Value
	Binary(op token.Token, y Value, side Side) (Value, error)
}

type Side bool

const (
	Left  Side = false
	Right Side = true
)

// A HasUnary value may be used as the operand of the unary operators. An
// implementation may decline to handle an operation by returning (nil, nil).
// For this reason, clients should always call the standalone Unary API
// function rather than calling the method directly.
type HasUnary interface {
	Value
	Unary(op token.Token) (Value, error)
}

// HasMetamap can be implemented by values that support customization of
// behavior via metamethods.
type HasMetamap interface {
	Value
	Metamap() *Map
	SetMetamap(*Map)
}

// A HasAttrs value has fields or methods that may be read by a dot expression
// (y = x.f). For implementation convenience, a result of (nil, nil) from Attr
// is interpreted as a "no such field or method" error. Implementations are
// free to return a more precise error.
type HasAttrs interface {
	Value
	// Attr returns the field or method value corresponding to the attribute
	// name. A return value of (nil, nil) is interpreted as a "no such field or
	// method" error.
	Attr(name string) (Value, error)
	// AttrNames returns a slice of strings of valid attribute names. The caller
	// must not modify the results.
	AttrNames() []string
}

// A HasSetField value has fields that may be written by a dot expression (x.f
// = y). An implementation of SetField may return a NoSuchAttrError, in which
// case the runtime may augment the error message to warn of possible
// misspelling.
type HasSetField interface {
	HasAttrs
	SetField(name string, val Value) error
}

// A NoSuchAttrError may be returned by an implementation of HasAttrs.Attr or
// HasSetField.SetField to indicate that no such field exists. In that case the
// runtime may augment the error message to warn of possible misspelling.
type NoSuchAttrError string

func (e NoSuchAttrError) Error() string { return string(e) }
