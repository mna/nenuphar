package ast

import (
	"fmt"

	"github.com/mna/nenuphar/lang/token"
)

type (
	// AssignStmt represents an assignment statement, e.g. x = y + z which may
	// also be a, b, c = 1, 2, 3 or an AugAssignStmt x += 2. It is also used to
	// represent DeclStmt.
	AssignStmt struct {
		DeclType    token.Token // zero if not a DeclStmt
		DeclStart   token.Pos   // zero if not a DeclStmt
		Left        []Expr      // only 1 for augassign
		LeftCommas  []token.Pos // always len(Left)-1, commas separating the Left
		AssignTok   token.Token // may be 0, either EQ or between PLUSEQ and GTGTEQ
		AssignPos   token.Pos   // may be 0, start pos of AssignTok
		Right       []Expr      // only 1 for augassign, may be empty for DeclStmt
		RightCommas []token.Pos // always len(Right)-1, commas separating the Right expressions
	}

	// BadStmt represents a bad statement that failed to parse.
	BadStmt struct {
		Start token.Pos
		End   token.Pos
	}

	// ClassStmt represents a class declaration statement.
	ClassStmt struct {
		Class    token.Pos
		Name     *IdentExpr
		Inherits *ClassInherit
		Body     *ClassBody
	}

	// ExprStmt represents an expression used as statement, which is only valid
	// for function calls (possibly wrapped in ParenExpr).
	ExprStmt struct {
		Expr Expr
	}

	// ForInStmt represents a for-in loop statement.
	ForInStmt struct {
		For         token.Pos
		Left        []Expr      // SuffixedExpr, has to be assignable
		LeftCommas  []token.Pos // always len(Left)-1, commas separating the Left
		In          token.Pos
		Right       []Expr
		RightCommas []token.Pos // always len(Right)-1, commas separating the Right expressions
		Do          token.Pos
		Body        *Block
	}

	// ForLoopStmt represents a 1- or 3-clause for loop statement.
	ForLoopStmt struct {
		For      token.Pos
		Init     Stmt      // may be nil, assign, augassign, decl or exprstmt (func call)
		InitSemi token.Pos // semicolon after init, may be zero
		Cond     Expr      // may be nil
		CondSemi token.Pos // semicolon after cond, may be zero
		Post     Stmt      // may be nil, assign, augassign or exprstmt (func call)
		Do       token.Pos
		Body     *Block
	}

	// FuncStmt represents a function declaration statement.
	FuncStmt struct {
		Fn   token.Pos
		Name *IdentExpr
		Sig  *FuncSignature
		Body *Block
		End  token.Pos
	}

	// IfGuardStmt represents an if..then..elseif..else or a guard..else
	// statement.
	IfGuardStmt struct {
		Type  token.Token // if, elseif or guard
		Start token.Pos   // Position of Type token
		Cond  Expr        // nil if bind-type statement
		Decl  *AssignStmt // nil if cond-type statement
		Then  token.Pos   // zero for guard
		True  *Block      // nil for guard
		Else  token.Pos   // zero if no else/elseif
		False *Block      // nil if no else, single stmt in block if elseif (an IfGuardStmt)
	}

	// LabelStmt represents a label declaration statement.
	LabelStmt struct {
		Lcolon token.Pos // start '::'
		Name   *IdentExpr
		Rcolon token.Pos // end '::'
	}

	// ReturnLikeStmt represents a return, break, continue, goto or throw.
	ReturnLikeStmt struct {
		Type  token.Token // return, break, continue, goto, throw
		Start token.Pos   // position of Type
		Expr  Expr        // may be nil, *Ident for break, continue, goto
	}

	// SimpleBlockStmt represents a simple keyword-defined block statement, do,
	// defer or catch.
	SimpleBlockStmt struct {
		Type  token.Token // do, defer, catch
		Start token.Pos   // position of Type
		Body  *Block
	}
)

func (n *AssignStmt) Format(f fmt.State, verb rune) {
	lbl := "assignment"
	if n.DeclType > 0 {
		lbl = n.DeclType.String() + " declaration"
	} else if n.AssignTok != token.EQ {
		lbl = "augmented assignment"
	}
	format(f, verb, n, lbl, map[string]int{"left": len(n.Left), "right": len(n.Right)})
}
func (n *AssignStmt) Span() (start, end token.Pos) {
	if n.DeclStart > 0 {
		start = n.DeclStart
	} else {
		start, _ = n.Left[0].Span()
	}
	_, end = n.Right[len(n.Right)-1].Span()
	return start, end
}
func (n *AssignStmt) Walk(v Visitor) {
	for _, e := range n.Left {
		Walk(v, e)
	}
	for _, e := range n.Right {
		Walk(v, e)
	}
}
func (n *AssignStmt) BlockEnding() bool { return false }

func (n *BadStmt) Format(f fmt.State, verb rune) {
	format(f, verb, n, "!bad stmt!", nil)
}
func (n *BadStmt) Span() (start, end token.Pos) {
	return n.Start, n.End
}
func (n *BadStmt) Walk(v Visitor)    {}
func (n *BadStmt) BlockEnding() bool { return false }

func (n *ClassStmt) Format(f fmt.State, verb rune) {
	var inheritsCount int
	if n.Inherits != nil {
		inheritsCount = 1
	}
	format(f, verb, n, "class decl", map[string]int{
		"inherits": inheritsCount,
		"methods":  len(n.Body.Methods),
		"fields":   len(n.Body.Fields),
	})
}
func (n *ClassStmt) Span() (start, end token.Pos) {
	return n.Class, n.Body.End + token.Pos(len(token.END.String()))
}
func (n *ClassStmt) Walk(v Visitor) {
	Walk(v, n.Name)
	if n.Inherits.Expr != nil {
		Walk(v, n.Inherits.Expr)
	}
	for _, e := range n.Body.Fields {
		Walk(v, e)
	}
	for _, e := range n.Body.Methods {
		Walk(v, e)
	}
}
func (n *ClassStmt) BlockEnding() bool { return false }

func (n *ExprStmt) Format(f fmt.State, verb rune) { format(f, verb, n, "expr", nil) }
func (n *ExprStmt) Span() (start, end token.Pos)  { return n.Expr.Span() }
func (n *ExprStmt) Walk(v Visitor)                { Walk(v, n.Expr) }
func (n *ExprStmt) BlockEnding() bool             { return false }

func (n *IfGuardStmt) Format(f fmt.State, verb rune) { format(f, verb, n, n.Type.String(), nil) }
func (n *IfGuardStmt) Span() (start, end token.Pos) {
	if n.True != nil {
		_, end = n.True.Span()
	}
	if n.False != nil {
		_, end = n.False.Span()
	}
	return n.Start, end
}
func (n *IfGuardStmt) Walk(v Visitor) {
	if n.Cond != nil {
		Walk(v, n.Cond)
	}
	if n.Decl != nil {
		Walk(v, n.Decl)
	}
	if n.True != nil {
		Walk(v, n.True)
	}
	if n.False != nil {
		Walk(v, n.False)
	}
}
func (n *IfGuardStmt) BlockEnding() bool { return false }

func (n *ForLoopStmt) Format(f fmt.State, verb rune) {
	var clauses int
	if n.Init != nil {
		clauses++
	}
	if n.Cond != nil {
		clauses++
	}
	if n.Post != nil {
		clauses++
	}
	format(f, verb, n, "for", map[string]int{"clauses": clauses})
}
func (n *ForLoopStmt) Span() (start, end token.Pos) {
	_, end = n.Body.Span()
	return n.For, end
}
func (n *ForLoopStmt) Walk(v Visitor) {
	if n.Init != nil {
		Walk(v, n.Init)
	}
	if n.Cond != nil {
		Walk(v, n.Cond)
	}
	if n.Post != nil {
		Walk(v, n.Post)
	}
	if n.Body != nil {
		Walk(v, n.Body)
	}
}
func (n *ForLoopStmt) BlockEnding() bool { return false }

func (n *ForInStmt) Format(f fmt.State, verb rune) {
	format(f, verb, n, "for in", map[string]int{"left": len(n.Left), "right": len(n.Right)})
}
func (n *ForInStmt) Span() (start, end token.Pos) {
	_, end = n.Body.Span()
	return n.For, end
}
func (n *ForInStmt) Walk(v Visitor) {
	for _, e := range n.Left {
		Walk(v, e)
	}
	for _, e := range n.Right {
		Walk(v, e)
	}
	if n.Body != nil {
		Walk(v, n.Body)
	}
}
func (n *ForInStmt) BlockEnding() bool { return false }

func (n *FuncStmt) Format(f fmt.State, verb rune) {
	lbl := "fn decl"
	if n.Sig.DotDotDot != 0 {
		lbl += " ..."
	}
	format(f, verb, n, lbl, map[string]int{"params": len(n.Sig.Params)})
}
func (n *FuncStmt) Span() (start, end token.Pos) {
	return n.Fn, n.End + token.Pos(len(token.END.String()))
}
func (n *FuncStmt) Walk(v Visitor) {
	Walk(v, n.Name)
	for _, e := range n.Sig.Params {
		Walk(v, e)
	}
	Walk(v, n.Body)
}
func (n *FuncStmt) BlockEnding() bool { return false }

func (n *LabelStmt) Format(f fmt.State, verb rune) { format(f, verb, n, "label", nil) }
func (n *LabelStmt) Span() (start, end token.Pos) {
	return n.Lcolon, n.Rcolon + token.Pos(len(token.COLONCOLON.String()))
}
func (n *LabelStmt) Walk(v Visitor) {
	Walk(v, n.Name)
}
func (n *LabelStmt) BlockEnding() bool { return false }

func (n *ReturnLikeStmt) Format(f fmt.State, verb rune) {
	var exprCount int
	if n.Expr != nil {
		exprCount = 1
	}
	format(f, verb, n, n.Type.String(), map[string]int{"expr": exprCount})
}
func (n *ReturnLikeStmt) Span() (start, end token.Pos) {
	end = n.Start + token.Pos(len(n.Type.String()))
	if n.Expr != nil {
		_, end = n.Expr.Span()
	}
	return n.Start, end
}
func (n *ReturnLikeStmt) Walk(v Visitor) {
	if n.Expr != nil {
		Walk(v, n.Expr)
	}
}
func (n *ReturnLikeStmt) BlockEnding() bool { return true }

func (n *SimpleBlockStmt) Format(f fmt.State, verb rune) { format(f, verb, n, n.Type.String(), nil) }
func (n *SimpleBlockStmt) Span() (start, end token.Pos) {
	_, end = n.Body.Span()
	return n.Start, end
}
func (n *SimpleBlockStmt) Walk(v Visitor) {
	if n.Body != nil {
		Walk(v, n.Body)
	}
}
func (n *SimpleBlockStmt) BlockEnding() bool { return false }
