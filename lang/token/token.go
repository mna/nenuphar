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

	// augmented binary operators
	PLUSEQ       // +=
	MINUSEQ      // -=
	STAREQ       // *=
	SLASHEQ      // /=
	SLASHSLASHEQ // //=
	PERCENTEQ    // %=
	CIRCUMFLEXEQ // ^=
	AMPERSANDEQ  // &=
	PIPEEQ       // |=
	TILDEEQ      // ~=
	LTLTEQ       // <<=
	GTGTEQ       // >>=

	// relational operators - order must match compiler.Opcode
	EQEQ   // ==
	BANGEQ // !=
	LT     // <
	GT     // >
	GE     // >=
	LE     // <=

	// punctuation
	SEMICOLON  // ;
	COMMA      // ,
	LBRACE     // {
	RBRACE     // }
	LBRACK     // [
	RBRACK     // ]
	LPAREN     // (
	RPAREN     // )
	COLON      // :
	DOT        // .
	BANG       // !
	EQ         // =
	COLONCOLON // ::

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

	PLUSEQ:       "+=",
	MINUSEQ:      "-=",
	STAREQ:       "*=",
	SLASHEQ:      "/=",
	SLASHSLASHEQ: "//=",
	PERCENTEQ:    "%=",
	CIRCUMFLEXEQ: "^=",
	AMPERSANDEQ:  "&=",
	PIPEEQ:       "|=",
	TILDEEQ:      "~=",
	LTLTEQ:       "<<=",
	GTGTEQ:       ">>=",

	EQEQ:   "==",
	BANGEQ: "!=",
	LT:     "<",
	GT:     ">",
	GE:     ">=",
	LE:     "<=",

	SEMICOLON:  ";",
	COMMA:      ",",
	LBRACE:     "{",
	RBRACE:     "}",
	LBRACK:     "[",
	RBRACK:     "]",
	LPAREN:     "(",
	RPAREN:     ")",
	COLON:      ":",
	DOT:        ".",
	BANG:       "!",
	EQ:         "=",
	COLONCOLON: "::",

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

var (
	keywords = func() map[string]Token {
		kw := make(map[string]Token)
		for i := kwStart + 1; i < kwEnd; i++ {
			kw[tokenNames[i]] = i
		}
		return kw
	}()
	punctuations = func() map[string]Token {
		puncts := make(map[string]Token)
		for i := punctStart + 1; i < punctEnd; i++ {
			puncts[tokenNames[i]] = i
		}
		return puncts
	}()
)

// LookupKw maps an identifier to its keyword token or IDENT (if not a
// keyword).
func LookupKw(ident string) Token {
	if tok, ok := keywords[ident]; ok {
		return tok
	}
	return IDENT
}

// LookupPunct maps a punctuation to its token or ILLEGAL (if not a valid
// punctuation).
func LookupPunct(punct string) Token {
	if tok, ok := punctuations[punct]; ok {
		return tok
	}
	return ILLEGAL
}
