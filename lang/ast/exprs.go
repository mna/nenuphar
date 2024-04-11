package ast

import (
	"fmt"

	"github.com/mna/nenuphar/lang/token"
)

// Unwrap the expression inside the parens. It unwraps multiple ParenExpr
// recursively until it reaches a non-ParenExpr.
func Unwrap(e Expr) Expr {
	if pe, ok := e.(*ParenExpr); ok {
		return Unwrap(pe)
	}
	return e
}

// IsValidStmt returns true if e is a valid ExprStmt expression. Only function
// calls, possibly prefixed with a "try" or "must" unary operator, are valid
// statements.
func IsValidStmt(e Expr) bool {
	ue := Unwrap(e)
	if unary, ok := ue.(*UnaryOpExpr); ok {
		if unary.Type != token.MUST && unary.Type != token.TRY {
			return false
		}
		ue = unary.Right
	}
	_, ok := ue.(*CallExpr)
	return ok
}

// IsAssignable returns true if e can be assigned to. For an expression to be
// assignable, it must be an IdentExpr, a DotExpr or an IndexExpr. Moreover,
// the left-hand side of those expressions must also be assignable.
func IsAssignable(e Expr) bool {
	switch e := e.(type) {
	case *IdentExpr:
		return true
	case *DotExpr:
		left := Unwrap(e.Left)
		return IsAssignable(left)
	case *IndexExpr:
		left := Unwrap(e.Prefix)
		return IsAssignable(left)
	default:
		return false
	}
}

type (
	// ArrayLikeExpr represents an array or tuple literal.
	ArrayLikeExpr struct {
		Type   token.Token // LBRACK or LPAREN
		Left   token.Pos
		Items  []Expr
		Commas []token.Pos // at least len(Items)-1, can be len(Items)
		Right  token.Pos
	}

	// BadExpr represents a bad expression that failed to parse.
	BadExpr struct {
		Start token.Pos
		End   token.Pos
	}

	// BinOpExpr represents a binary expression, e.g. x + y.
	BinOpExpr struct {
		Left  Expr
		Type  token.Token // binary operator token type
		Op    token.Pos
		Right Expr
	}

	// CallExpr represents a function call, e.g. x(y, z).
	CallExpr struct {
		Fn     Expr
		Bang   token.Pos // 0 if no '!'
		Lparen token.Pos // 0 if '!' or map/string single arg
		Args   []Expr
		Commas []token.Pos // len(Args)-1
		Rparen token.Pos   // 0 if '!' or map/string single arg
	}

	// ClassExpr represents a class literal.
	ClassExpr struct {
		Class    token.Pos
		Inherits *ClassInherit
		Body     *ClassBody
	}

	// DotExpr represents a selector expression e.g. x.y.
	DotExpr struct {
		Left  Expr
		Dot   token.Pos
		Right *IdentExpr
	}

	// FuncExpr represents a function literal.
	FuncExpr struct {
		Fn   token.Pos
		Sig  *FuncSignature
		Body *Block
		End  token.Pos
	}

	// IdentExpr represents an identifier.
	IdentExpr struct {
		Start token.Pos
		Lit   string
	}

	// IndexExpr represents an index expression e.g. x[y].
	IndexExpr struct {
		Prefix Expr
		Lbrack token.Pos
		Index  Expr
		Rbrack token.Pos
	}

	// LiteralExpr represents a literal string or number.
	LiteralExpr struct {
		Type  token.Token // null, true, false, string, int or float
		Start token.Pos
		Raw   string      // uninterpreted text
		Value interface{} // = string | int64 | float64 (nil for null/true/false)
	}

	// MapExpr represents a map literal.
	MapExpr struct {
		Lbrace token.Pos
		Items  []*KeyVal
		Commas []token.Pos // at least len(Items)-1, can be len(Items)
		Rbrace token.Pos
	}

	// ParenExpr represents an expression wrapped in parentheses.
	ParenExpr struct {
		Lparen token.Pos
		Expr   Expr
		Rparen token.Pos
	}

	// UnaryOpExpr represents a unary operator expression (e.g. -4).
	UnaryOpExpr struct {
		Type  token.Token // unary operator token type
		Op    token.Pos
		Right Expr
	}
)

func (n *ArrayLikeExpr) Format(f fmt.State, verb rune) {
	lbl := "array"
	if n.Type == token.LPAREN {
		lbl = "tuple"
	}
	format(f, verb, n, lbl, map[string]int{"items": len(n.Items)})
}
func (n *ArrayLikeExpr) Span() (start, end token.Pos) {
	return n.Left, n.Right + token.Pos(len(n.Type.String()))
}
func (n *ArrayLikeExpr) Walk(v Visitor) {
	for _, e := range n.Items {
		Walk(v, e)
	}
}
func (n *ArrayLikeExpr) expr() {}

func (n *BadExpr) Format(f fmt.State, verb rune) {
	format(f, verb, n, "!bad expr!", nil)
}
func (n *BadExpr) Span() (start, end token.Pos) {
	return n.Start, n.End
}
func (n *BadExpr) Walk(v Visitor) {}
func (n *BadExpr) expr()          {}

func (n *BinOpExpr) Format(f fmt.State, verb rune) {
	format(f, verb, n, "binary "+n.Type.GoString(), nil)
}
func (n *BinOpExpr) Span() (start, end token.Pos) {
	start, _ = n.Left.Span()
	_, end = n.Right.Span()
	return start, end
}
func (n *BinOpExpr) Walk(v Visitor) {
	Walk(v, n.Left)
	Walk(v, n.Right)
}
func (n *BinOpExpr) expr() {}

func (n *CallExpr) Format(f fmt.State, verb rune) {
	format(f, verb, n, "call", map[string]int{"args": len(n.Args)})
}
func (n *CallExpr) Span() (start, end token.Pos) {
	start, _ = n.Fn.Span()
	if n.Bang.IsValid() {
		end = n.Bang + token.Pos(len(token.BANG.String()))
	} else if n.Rparen.IsValid() {
		end = n.Rparen + token.Pos(len(token.RPAREN.String()))
	} else {
		_, end = n.Args[len(n.Args)-1].Span()
	}
	return start, end
}
func (n *CallExpr) Walk(v Visitor) {
	Walk(v, n.Fn)
	for _, e := range n.Args {
		Walk(v, e)
	}
}
func (n *CallExpr) expr() {}

func (n *ClassExpr) Format(f fmt.State, verb rune) {
	var inheritsCount int
	if n.Inherits != nil {
		inheritsCount = 1
	}
	format(f, verb, n, "class", map[string]int{
		"inherits": inheritsCount,
		"methods":  len(n.Body.Methods),
		"fields":   len(n.Body.Fields),
	})
}
func (n *ClassExpr) Span() (start, end token.Pos) {
	return n.Class, n.Body.End + token.Pos(len(token.END.String()))
}
func (n *ClassExpr) Walk(v Visitor) {
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
func (n *ClassExpr) expr() {}

func (n *DotExpr) Format(f fmt.State, verb rune) {
	format(f, verb, n, "expr.ident", nil)
}
func (n *DotExpr) Span() (start, end token.Pos) {
	start, _ = n.Left.Span()
	_, end = n.Right.Span()
	return start, end
}
func (n *DotExpr) Walk(v Visitor) {
	Walk(v, n.Left)
	Walk(v, n.Right)
}
func (n *DotExpr) expr() {}

func (n *FuncExpr) Format(f fmt.State, verb rune) {
	lbl := "fn"
	if n.Sig.DotDotDot.IsValid() {
		lbl += " ..."
	}
	format(f, verb, n, lbl, map[string]int{"params": len(n.Sig.Params)})
}
func (n *FuncExpr) Span() (start, end token.Pos) {
	return n.Fn, n.End + token.Pos(len(token.END.String()))
}
func (n *FuncExpr) Walk(v Visitor) {
	for _, e := range n.Sig.Params {
		Walk(v, e)
	}
	Walk(v, n.Body)
}
func (n *FuncExpr) expr() {}

func (n *IdentExpr) Format(f fmt.State, verb rune) {
	format(f, verb, n, n.Lit, nil)
}
func (n *IdentExpr) Span() (start, end token.Pos) {
	return n.Start, n.Start + token.Pos(len(n.Lit))
}
func (n *IdentExpr) Walk(v Visitor) {}
func (n *IdentExpr) expr()          {}

func (n *IndexExpr) Format(f fmt.State, verb rune) {
	format(f, verb, n, "expr[index]", nil)
}
func (n *IndexExpr) Span() (start, end token.Pos) {
	start, _ = n.Prefix.Span()
	return start, n.Rbrack + token.Pos(len(token.RBRACK.String()))
}
func (n *IndexExpr) Walk(v Visitor) {
	Walk(v, n.Prefix)
	Walk(v, n.Index)
}
func (n *IndexExpr) expr() {}

func (n *LiteralExpr) Format(f fmt.State, verb rune) {
	format(f, verb, n, n.Type.String(), nil)
	if n.Value == nil {
		format(f, verb, n, n.Type.String(), nil)
	} else {
		format(f, verb, n, n.Type.String()+" "+n.Raw, nil)
	}
}
func (n *LiteralExpr) Span() (start, end token.Pos) {
	return n.Start, n.Start + token.Pos(len(n.Raw))
}
func (n *LiteralExpr) Walk(v Visitor) {}
func (n *LiteralExpr) expr()          {}

func (n *MapExpr) Format(f fmt.State, verb rune) {
	format(f, verb, n, "map", map[string]int{"keyvals": len(n.Items)})
}
func (n *MapExpr) Span() (start, end token.Pos) {
	return n.Lbrace, n.Rbrace + token.Pos(len(token.RBRACE.String()))
}
func (n *MapExpr) Walk(v Visitor) {
	for _, kv := range n.Items {
		Walk(v, kv.Key)
		Walk(v, kv.Value)
	}
}
func (n *MapExpr) expr() {}

func (n *ParenExpr) Format(f fmt.State, verb rune) {
	format(f, verb, n, "(expr)", nil)
}
func (n *ParenExpr) Span() (start, end token.Pos) {
	return n.Lparen, n.Rparen + token.Pos(len(token.RPAREN.String()))
}
func (n *ParenExpr) Walk(v Visitor) {
	Walk(v, n.Expr)
}
func (n *ParenExpr) expr() {}

func (n *UnaryOpExpr) Format(f fmt.State, verb rune) {
	format(f, verb, n, "unary "+n.Type.GoString(), nil)
}
func (n *UnaryOpExpr) Span() (start, end token.Pos) {
	_, end = n.Right.Span()
	return n.Op, end
}
func (n *UnaryOpExpr) Walk(v Visitor) {
	Walk(v, n.Right)
}
func (n *UnaryOpExpr) expr() {}
