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
	STRING // "foo" or 'foo' or '''foo''' or r'foo' or r"foo"
	BYTES  // b"foo", etc

	// Punctuation
	PLUS          // +
	MINUS         // -
	STAR          // *
	SLASH         // /
	SLASHSLASH    // //
	PERCENT       // %
	AMPERSAND     // &
	PIPE          // |
	CIRCUMFLEX    // ^
	LTLT          // <<
	GTGT          // >>
	TILDE         // ~
	DOT           // .
	COMMA         // ,
	EQ            // =
	SEMI          // ;
	COLON         // :
	LPAREN        // (
	RPAREN        // )
	LBRACK        // [
	RBRACK        // ]
	LBRACE        // {
	RBRACE        // }
	LT            // <
	GT            // >
	GE            // >=
	LE            // <=
	EQL           // ==
	NEQ           // !=
	PLUS_EQ       // +=    (keep order consistent with PLUS..GTGT)
	MINUS_EQ      // -=
	STAR_EQ       // *=
	SLASH_EQ      // /=
	SLASHSLASH_EQ // //=
	PERCENT_EQ    // %=
	AMP_EQ        // &=
	PIPE_EQ       // |=
	CIRCUMFLEX_EQ // ^=
	LTLT_EQ       // <<=
	GTGT_EQ       // >>=
	STARSTAR      // **

	// Keywords
	AND
	BREAK
	CONTINUE
	DEF
	ELIF
	ELSE
	FOR
	IF
	IN
	LAMBDA
	LOAD
	NOT
	NOT_IN // synthesized by parser from NOT IN
	OR
	PASS
	RETURN
	WHILE

	maxToken
)

func (tok Token) String() string { return tokenNames[tok] }

// GoString is like String but quotes punctuation tokens. Use Sprintf("%#v",
// tok) when constructing error messages.
func (tok Token) GoString() string {
	if tok >= PLUS && tok <= STARSTAR {
		return "'" + tokenNames[tok] + "'"
	}
	return tokenNames[tok]
}

var tokenNames = [...]string{
	ILLEGAL:       "illegal token",
	EOF:           "end of file",
	IDENT:         "identifier",
	INT:           "int literal",
	FLOAT:         "float literal",
	STRING:        "string literal",
	PLUS:          "+",
	MINUS:         "-",
	STAR:          "*",
	SLASH:         "/",
	SLASHSLASH:    "//",
	PERCENT:       "%",
	AMPERSAND:     "&",
	PIPE:          "|",
	CIRCUMFLEX:    "^",
	LTLT:          "<<",
	GTGT:          ">>",
	TILDE:         "~",
	DOT:           ".",
	COMMA:         ",",
	EQ:            "=",
	SEMI:          ";",
	COLON:         ":",
	LPAREN:        "(",
	RPAREN:        ")",
	LBRACK:        "[",
	RBRACK:        "]",
	LBRACE:        "{",
	RBRACE:        "}",
	LT:            "<",
	GT:            ">",
	GE:            ">=",
	LE:            "<=",
	EQL:           "==",
	NEQ:           "!=",
	PLUS_EQ:       "+=",
	MINUS_EQ:      "-=",
	STAR_EQ:       "*=",
	SLASH_EQ:      "/=",
	SLASHSLASH_EQ: "//=",
	PERCENT_EQ:    "%=",
	AMP_EQ:        "&=",
	PIPE_EQ:       "|=",
	CIRCUMFLEX_EQ: "^=",
	LTLT_EQ:       "<<=",
	GTGT_EQ:       ">>=",
	STARSTAR:      "**",
	AND:           "and",
	BREAK:         "break",
	CONTINUE:      "continue",
	DEF:           "def",
	ELIF:          "elif",
	ELSE:          "else",
	FOR:           "for",
	IF:            "if",
	IN:            "in",
	LAMBDA:        "lambda",
	LOAD:          "load",
	NOT:           "not",
	NOT_IN:        "not in",
	OR:            "or",
	PASS:          "pass",
	RETURN:        "return",
	WHILE:         "while",
}
