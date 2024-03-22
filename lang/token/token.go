package token

// A Token represents a lexical token.
type Token int8

//nolint:revive
const (
	ILLEGAL Token = iota
	EOF

	// Tokens with values
	IDENT  // x
	INT    // 123
	FLOAT  // 1.23e45
	STRING // "foo" or 'foo' or [[foo]]

	// Punctuation

	// operators - order must match compiler.Opcode
	PLUS       // +
	MINUS      // -
	STAR       // *
	SLASH      // /
	SLASHSLASH // //
	PERCENT    // %
	CIRCUMFLEX // ^
	AMPERSAND  // &
	PIPE       // |
	TILDE      // ~
	LTLT       // <<
	GTGT       // >>
	POUND      // #
	DOTDOTDOT  // ...

	// TODO: += -= etc.?

	// relational operators - order must match compiler.Opcode
	EQEQ          // ==
	EXCLAMATIONEQ // !=
	LT            // <
	GT            // >
	GE            // >=
	LE            // <=

	// punctuation
	SEMICOLON   // ;
	COMMA       // ,
	LBRACE      // {
	RBRACE      // }
	LBRACK      // [
	RBRACK      // ]
	LPAREN      // (
	RPAREN      // )
	COLON       // :
	DOT         // .
	EXCLAMATION // !
	EQ          // =
	COLONCOLON  // ::

	// Keywords
	FUNCTION
	CLASS
	NULL // TODO: use universe lookup to resolve null, true, false and import?
	TRUE
	FALSE
	END
	IF
	THEN
	ELSEIF
	ELSE
	GUARD
	DO
	FOR
	IN
	DEFER
	CATCH
	THROW
	LET
	CONST
	RETURN
	BREAK
	CONTINUE
	GOTO
	AND
	OR
	NOT
	TRY
	MUST

	maxToken             = MUST
	litStart, litEnd     = IDENT, STRING
	punctStart, punctEnd = PLUS, COLONCOLON
	kwStart, kwEnd       = FUNCTION, MUST
)

func (tok Token) String() string { return tokenNames[tok] }

// GoString is like String but quotes punctuation tokens. Use Sprintf("%#v",
// tok) when constructing error messages.
func (tok Token) GoString() string {
	if tok >= punctStart && tok <= punctEnd {
		return "'" + tokenNames[tok] + "'"
	}
	return tokenNames[tok]
}

var tokenNames = [...]string{
	ILLEGAL: "illegal token",
	EOF:     "end of file",

	IDENT:  "identifier",
	INT:    "int literal",
	FLOAT:  "float literal",
	STRING: "string literal",

	PLUS:       "+",
	MINUS:      "-",
	STAR:       "*",
	SLASH:      "/",
	SLASHSLASH: "//",
	PERCENT:    "%",
	CIRCUMFLEX: "^",
	AMPERSAND:  "&",
	PIPE:       "|",
	TILDE:      "~",
	LTLT:       "<<",
	GTGT:       ">>",
	POUND:      "#",
	DOTDOTDOT:  "...",

	EQEQ:          "==",
	EXCLAMATIONEQ: "!=",
	LT:            "<",
	GT:            ">",
	GE:            ">=",
	LE:            "<=",

	SEMICOLON:   ";",
	COMMA:       ",",
	LBRACE:      "{",
	RBRACE:      "}",
	LBRACK:      "[",
	RBRACK:      "]",
	LPAREN:      "(",
	RPAREN:      ")",
	COLON:       ":",
	DOT:         ".",
	EXCLAMATION: "!",
	EQ:          "=",
	COLONCOLON:  "::",

	FUNCTION: "fn",
	CLASS:    "class",
	NULL:     "null",
	TRUE:     "true",
	FALSE:    "false",
	END:      "end",
	IF:       "if",
	THEN:     "then",
	ELSEIF:   "elseif",
	ELSE:     "else",
	GUARD:    "guard",
	DO:       "do",
	FOR:      "for",
	IN:       "in",
	DEFER:    "defer",
	CATCH:    "catch",
	THROW:    "throw",
	LET:      "let",
	CONST:    "const",
	RETURN:   "return",
	BREAK:    "break",
	CONTINUE: "continue",
	GOTO:     "goto",
	AND:      "and",
	OR:       "or",
	NOT:      "not",
	TRY:      "try",
	MUST:     "must",
}
