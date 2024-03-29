package ast

// VisitDirection indicates whether a call to Visit enters or exits a node.
type VisitDirection int

// List of visit directions.
const (
	VisitEnter VisitDirection = iota
	VisitExit
)

// Visitor defines the method to implement for a Visitor, which gets called
// for each participating node in the call to Walk. A node's children can
// be skipped by returning a nil visitor from the call to Visit.
type Visitor interface {
	Visit(n Node, dir VisitDirection) (w Visitor)
}

// VisitorFunc is a function that implements the Visitor interface.
type VisitorFunc func(n Node, dir VisitDirection) Visitor

// Visit implements the Visitor interface for VisitorFunc.
func (f VisitorFunc) Visit(n Node, dir VisitDirection) Visitor {
	return f(n, dir)
}

// Walk visits each node with Visitor v starting with the provided node. It
// first calls Visit with the node in VisitEnter direction, and if that call
// returns a non-nil Visitor, it recursively walks the children of this node
// and calls Visit again with the node and VisitExit direction when it exits
// the node (after all children have been visited).
func Walk(v Visitor, node Node) {
	if v = v.Visit(node, VisitEnter); v == nil {
		return
	}
	node.Walk(v)
	v.Visit(node, VisitExit)
}
