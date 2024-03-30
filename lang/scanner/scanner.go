// Some of the scanner package is adapted from the Go source code:
// https://cs.opensource.google/go/go/+/refs/tags/go1.22.1:src/go/scanner/scanner.go
//
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package scanner

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"go/scanner"
	"os"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/mna/nenuphar/lang/token"
)

type (
	Error     = scanner.Error
	ErrorList = scanner.ErrorList
)

var PrintError = scanner.PrintError

// TokenAndValue combines the token type with the token value type in the same
// struct.
type TokenAndValue struct {
	Token token.Token
	Value token.Value
}

// ScanFiles is a helper function that tokenizes the source files and returns
// the list of tokens, grouped by the file at the same index, and produces any
// error encountered. The error, if non-nil, is guaranteed to implement
// Unwrap() []error.
func ScanFiles(ctx context.Context, files ...string) (*token.FileSet, [][]TokenAndValue, error) {
	if len(files) == 0 {
		return nil, nil, nil
	}

	var (
		s      Scanner
		tokVal token.Value
		el     ErrorList
	)

	fs := token.NewFileSet()
	tokensByFile := make([][]TokenAndValue, len(files))
	for i, file := range files {
		b, err := os.ReadFile(file)
		if err != nil {
			el.Add(token.Position{Filename: file}, err.Error())
			continue
		}

		fsf := fs.AddFile(file, -1, len(b))
		s.Init(fsf, b, el.Add)
		for {
			tok := s.Scan(&tokVal)
			tokensByFile[i] = append(tokensByFile[i], TokenAndValue{
				Token: tok,
				Value: tokVal,
			})
			if tok == token.EOF {
				break
			}
		}
	}
	el.Sort()
	return fs, tokensByFile, el.Err()
}

// Scanner tokenizes source files for the parser to consume.
type Scanner struct {
	// immutable state after Init
	file *token.File // source file handle
	src  []byte
	err  func(pos token.Position, msg string)

	// mutable scanning state
	sb               strings.Builder // writes to Builder never fail, so errors are ignored
	pendingSurrogate rune            // in short string literal, the first half of a surrogate pair, pending the second (or rendered as replacement rune)
	invalidByte      byte            // when cur==RuneError due to failed utf8 decode, this is the invalid byte
	cur              rune            // current character
	off              int             // character offset in bytes of cur
	roff             int             // reading offset in bytes (position after current character)
}

var (
	// byte order mark, only permitted as very first characters
	bom = [2]byte{0xFE, 0xFF}
	// hashbang line, only permitted as very first line (or immediately after
	// bom)
	hashBang = [2]byte{'#', '!'}
)

// Init initializes the scanner to tokenize a new file. It panics if the file
// size is not the same as the length of the src slice.
func (s *Scanner) Init(file *token.File, src []byte, errHandler func(token.Position, string)) {
	if file.Size() != len(src) {
		panic(fmt.Sprintf("file size (%d) does not match src len (%d)", file.Size(), len(src)))
	}

	s.file = file
	s.src = src
	s.err = errHandler

	s.sb.Reset()
	s.pendingSurrogate = 0
	s.invalidByte = 0
	s.cur = ' '
	s.off = 0
	s.roff = 0

	// skip initial BOM if present
	if len(src) >= len(bom) && bytes.Equal(src[:len(bom)], bom[:]) {
		s.off += len(bom)
		s.roff += len(bom)
	}
	// skip initial hashbang line if present
	if len(src)-s.roff >= len(hashBang) && bytes.Equal(src[s.roff:s.roff+len(hashBang)], hashBang[:]) {
		for s.cur != '\n' && s.cur != -1 {
			s.advance()
		}
	}
	s.advance()
}

// peek returns the byte following the most recently read character without
// advancing the scanner. If the scanner is at EOF, peek returns 0.
func (s *Scanner) peek() byte {
	if s.roff < len(s.src) {
		return s.src[s.roff]
	}
	return 0
}

// read the next Unicode char into s.cur; s.cur < 0 means end-of-file.
func (s *Scanner) advance() {
	if s.roff >= len(s.src) {
		s.off = len(s.src)
		if s.cur == '\n' {
			s.file.AddLine(s.off)
		}
		s.cur = -1
		return
	}

	s.off = s.roff
	if s.cur == '\n' {
		s.file.AddLine(s.off)
	}

	// fast path if the rune is an ASCII char, no decoding necessary
	s.invalidByte = 0
	r, w := rune(s.src[s.roff]), 1
	if r >= utf8.RuneSelf {
		// not ASCII
		r, w = utf8.DecodeRune(s.src[s.roff:])
		if r == utf8.RuneError && w == 1 {
			s.error(s.off, "illegal UTF-8 encoding")
			// store the actual invalid byte
			s.invalidByte = s.src[s.roff]
		}
	}
	s.roff += w
	s.cur = r
}

func (s *Scanner) error(off int, msg string) {
	if s.err != nil {
		s.err(s.file.Position(s.file.Pos(off)), msg)
	}
}

func (s *Scanner) errorf(off int, format string, args ...any) {
	s.error(off, fmt.Sprintf(format, args...))
}

// advance only if the current char matches any of the specified ones.
func (s *Scanner) advanceIf(matches ...byte) bool {
	if bytes.ContainsRune(matches, s.cur) {
		s.advance()
		return true
	}
	return false
}

// Scan returns the next token in the source file.
func (s *Scanner) Scan(tokVal *token.Value) (tok token.Token) {
	s.skipWhitespace()

	// current token start
	pos := s.file.Pos(s.off)
	start := s.off

	switch cur := s.cur; {
	case isLetter(cur):
		// keywords and identifiers
		lit := s.ident()
		tok = token.IDENT
		if len(lit) > 1 {
			// keywords are longer than one letter - avoid lookup otherwise
			tok = token.LookupKw(lit)
		}
		*tokVal = token.Value{Raw: lit, Pos: pos}

	case isDecimal(cur) || cur == '.' && isDecimal(rune(s.peek())):
		// integer and float
		var base int
		var lit string
		tok, base, lit = s.number()
		*tokVal = token.Value{Raw: lit, Pos: pos}
		if tok == token.INT {
			v, err := numberToInt(lit, base)
			if err != nil && errors.Is(err, strconv.ErrRange) {
				// syntax errors would have already generated an error, but not range
				s.error(start, "integer literal value out of range")
			}
			tokVal.Int = v
		} else if tok == token.FLOAT {
			v, err := numberToFloat(lit)
			if err != nil && errors.Is(err, strconv.ErrRange) {
				// syntax errors would have already generated an error, but not range
				s.error(start, "float literal value out of range")
			}
			tokVal.Float = v
		}

	default:
		// keywords, identifiers and numbers are done

		s.advance() // always make progress
		switch cur {
		case '=':
			tok = token.EQ
			if s.advanceIf('=') {
				tok = token.EQEQ
			}
			*tokVal = token.Value{Raw: tok.String(), Pos: pos}

		case '"', '\'':
			// short string
			tok = token.STRING
			lit, val := s.shortString(cur)
			*tokVal = token.Value{Raw: lit, Pos: pos, String: val}

		case '[':
			// can be Lbrack or long String
			if s.cur == '=' || s.cur == '[' {
				tok = token.STRING
				lit, val := s.longString()
				*tokVal = token.Value{Raw: lit, Pos: pos, String: val}
				break
			}
			tok = token.LBRACK

		case '(', ')', ',', '{', '}', ']', '#', ';':
			// unambiguous single-char punctuation
			tok = token.LookupPunct(string(cur))
			*tokVal = token.Value{Raw: tok.String(), Pos: pos}

		case '+', '*', '!', '%', '^', '&', '|', '~':
			// single-char operators that can be followed by '=' and nothing else
			if s.advanceIf('=') {
				tok = token.LookupPunct(string(s.src[start:s.off]))
			} else {
				tok = token.LookupPunct(string(cur))
			}
			*tokVal = token.Value{Raw: tok.String(), Pos: pos}

		case '-':
			// minus, minuseq or start of a comment (--)
			tok = token.MINUS
			if s.advanceIf('=') {
				tok = token.MINUSEQ
			} else if s.advanceIf('-') {
				tok = token.COMMENT
				lit, val := s.comment()
				*tokVal = token.Value{Raw: lit, Pos: pos, String: val}
				break
			}
			*tokVal = token.Value{Raw: tok.String(), Pos: pos}

		case '<', '>', '/':
			// all can be followed by the same, eq or the same and eq
			s.advanceIf(byte(cur))
			s.advanceIf('=')
			tok = token.LookupPunct(string(s.src[start:s.off]))
			*tokVal = token.Value{Raw: tok.String(), Pos: pos}

		case ':':
			// colon or colon colon
			tok = token.COLON
			if s.advanceIf('.') {
				tok = token.COLONCOLON
			}
			*tokVal = token.Value{Raw: tok.String(), Pos: pos}

		case '.':
			// dot or dotdotdot
			tok = token.DOT
			raw := tok.String()
			if s.advanceIf('.') {
				if s.advanceIf('.') {
					tok = token.DOTDOTDOT
					raw = tok.String()
				} else {
					// we could tokenize this as DOT and DOT, but it's never a valid
					// sequence so we error (and we only have 1 lookahead).
					s.error(start, "illegal punctuation '..'")
					tok = token.ILLEGAL
					raw = ".."
				}
			}
			*tokVal = token.Value{Raw: raw, Pos: pos}

		case -1:
			tok = token.EOF
			*tokVal = token.Value{Raw: "", Pos: pos}

		default:
			if cur == utf8.RuneError && s.invalidByte > 0 {
				cur = rune(s.invalidByte)
				s.invalidByte = 0
			}
			s.errorf(start, "illegal character %#U", cur)
			tok = token.ILLEGAL
			*tokVal = token.Value{Raw: string(cur), Pos: pos}
		}
	}
	return tok
}

func (s *Scanner) ident() string {
	start := s.off
	for isLetter(s.cur) || isDigit(s.cur) {
		s.advance()
	}
	return string(s.src[start:s.off])
}

func (s *Scanner) skipWhitespace() {
	for isWhitespace(s.cur) {
		s.advance()
	}
}

func isWhitespace(rn rune) bool {
	return rn == ' ' || rn == '\t' || rn == '\n' || rn == '\r'
}

func isLetter(rn rune) bool {
	return 'a' <= rn && rn <= 'z' ||
		'A' <= rn && rn <= 'Z' ||
		rn == '_' ||
		rn >= utf8.RuneSelf && unicode.IsLetter(rn)
}

func isDigit(rn rune) bool {
	return '0' <= rn && rn <= '9' ||
		rn >= utf8.RuneSelf && unicode.IsDigit(rn)
}
