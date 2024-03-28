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
	"os"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/mna/nenuphar/lang/token"
)

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
func ScanFiles(ctx context.Context, files ...string) ([][]TokenAndValue, error) {
	if len(files) == 0 {
		return nil, nil
	}

	var (
		s      Scanner
		tokVal token.Value
		errs   []error
	)

	tokensByFile := make([][]TokenAndValue, len(files))
	for i, file := range files {
		b, err := os.ReadFile(file)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", token.MakePosition(file, 0, 0, 0), err))
			continue
		}

		s.Init(file, b, func(pos token.Position, msg string) {
			errs = append(errs, fmt.Errorf("%s: %s", pos, msg))
		})
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
	return tokensByFile, errors.Join(errs...)
}

// Scanner tokenizes source files for the parser to consume.
type Scanner struct {
	// immutable state after Init
	filename string
	src      []byte
	err      func(pos token.Position, msg string) // error handler for scanning errors

	// mutable scanning state
	sb               strings.Builder // writes to Builder never fail, so errors are ignored
	pendingSurrogate rune            // in short string literal, the first half of a surrogate pair, pending the second (or rendered as replacement rune)
	invalidByte      byte            // when cur==RuneError due to failed utf8 decode, this is the invalid byte
	cur              rune            // current character
	line, col        int             // line/col position of cur
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
func (s *Scanner) Init(filename string, src []byte, errHandler func(token.Position, string)) {
	s.filename = filename
	s.src = src
	s.err = errHandler

	s.sb.Reset()
	s.pendingSurrogate = 0
	s.invalidByte = 0
	s.cur = ' '
	s.line, s.col = 1, 0
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
			s.line++
			s.col = 0
		}
		s.cur = -1
		return
	}

	s.off = s.roff
	if s.cur == '\n' {
		s.line++
		s.col = 0
	}

	// fast path if the rune is an ASCII char, no decoding necessary
	s.invalidByte = 0
	r, w := rune(s.src[s.roff]), 1
	if r >= utf8.RuneSelf {
		// not ASCII
		r, w = utf8.DecodeRune(s.src[s.roff:])
		if r == utf8.RuneError && w == 1 {
			s.error(s.roff, s.line, s.col, "illegal UTF-8 encoding")
			// store the actual invalid byte
			s.invalidByte = s.src[s.roff]
		}
	}
	s.roff += w
	s.cur = r
	s.col++
}

func (s *Scanner) error(off, line, col int, msg string) {
	checkSafePos(line, col)
	s.err(token.MakePosition(s.filename, off, line, col), msg)
}

func (s *Scanner) errorf(off, line, col int, msg string, args ...any) {
	s.error(off, line, col, fmt.Sprintf(msg, args...))
}

func checkSafePos(line, col int) {
	if line > token.MaxLines || col > token.MaxCols {
		if line > token.MaxLines {
			panic(fmt.Sprintf("number of lines exceeded: %d", line))
		}
		panic(fmt.Sprintf("number of columns exceeded at line %d: %d", line, col))
	}
}

func makeSafePos(line, col int) token.Pos {
	checkSafePos(line, col)
	return token.MakePos(line, col)
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
	startOff, startLine, startCol := s.off, s.line, s.col

	switch cur := s.cur; {
	case isLetter(cur):
		// keywords and identifiers
		lit := s.ident()
		tok = token.IDENT
		if len(lit) > 1 {
			// keywords are longer than one letter - avoid lookup otherwise
			tok = token.LookupKw(lit)
		}
		*tokVal = token.Value{Raw: lit, Pos: makeSafePos(startLine, startCol)}

	case isDecimal(cur) || cur == '.' && isDecimal(rune(s.peek())):
		// integer and float
		var base int
		var lit string
		tok, base, lit = s.number()
		*tokVal = token.Value{Raw: lit, Pos: makeSafePos(startLine, startCol)}
		if tok == token.INT {
			tokVal.Int = numberToInt(lit, base)
		} else if tok == token.FLOAT {
			tokVal.Float = numberToFloat(lit)
		}

	default:
		// keywords, identifiers and numbers are done

		s.advance() // always make progress
		switch cur {
		case '"', '\'':
			// short string
			tok = token.STRING
			lit, val := s.shortString(cur)
			*tokVal = token.Value{Raw: lit, Pos: makeSafePos(startLine, startCol), String: val}

		case '[':
			// can be Lbrack or long String
			if s.cur == '=' || s.cur == '[' {
				tok = token.STRING
				lit, val := s.longString()
				*tokVal = token.Value{Raw: lit, Pos: makeSafePos(startLine, startCol), String: val}
				break
			}
			tok = token.LBRACK

		case ';', ',', '{', '}', ']', '(', ')':
			// unambiguous single-char punctuation
			tok = token.LookupPunct(string(cur))
			*tokVal = token.Value{Raw: tok.String(), Pos: makeSafePos(startLine, startCol)}

		case -1:
			tok = token.EOF

			/*
				case '+', '*', '^', '%', '&', '~', '|', '#', ';', ',', '(', ')', '{', '}', ']':
					// all unambiguous single-char operators/delimiters can be processed here
					tok = token.LookupOp(string(cur))

				case '-':
					// can be Sub or Comment
					if s.advanceIf('-') {
						tok = token.Comment
						lit = s.comment(start)
						break
					}
					tok = token.Sub

				case '/':
					// can be Div or FloorDiv
					switch {
					case s.advanceIf('/'):
						tok = token.FloorDiv
					default:
						tok = token.Div
					}

				case '!':
					// can be Bang or NotEq
					if s.advanceIf('=') {
						tok = token.NotEq
						break
					}
					tok = token.Bang

				case '=':
					// can be Assign or Eq
					if s.advanceIf('=') {
						tok = token.Eq
						break
					}
					tok = token.Assign

				case ':':
					// can be Colon or ColonColon
					if s.advanceIf(':') {
						tok = token.ColonColon
						break
					}
					tok = token.Colon

				case '>':
					// can be Gt, Gte or ShiftRight
					switch {
					case s.advanceIf('='):
						tok = token.Gte
					case s.advanceIf('>'):
						tok = token.ShiftRight
					default:
						tok = token.Gt
					}

				case '<':
					// can be Lt, Lte or ShiftLeft
					switch {
					case s.advanceIf('='):
						tok = token.Lte
					case s.advanceIf('<'):
						tok = token.ShiftLeft
					default:
						tok = token.Lt
					}

				case '.':
					// can be Dot, Concat or Unpack, or the start of a Float
					tok = token.Dot
					if s.advanceIf('.') {
						tok = token.Concat
						if s.advanceIf('.') {
							tok = token.Unpack
						}
					} else if isDecimal(s.cur) {
						tok, lit = s.number(start)
					}

				case '?':
					if s.advanceIf('.') {
						tok = token.QuestionDot
						break
					} else if s.advanceIf('#') {
						tok = token.QuestionPound
						break
					}
					s.errorf(s.file.Offset(pos), "illegal character %#U", cur)
					tok = token.Illegal
					lit = string(cur)
			*/

		default:
			if cur == utf8.RuneError && s.invalidByte > 0 {
				cur = rune(s.invalidByte)
				s.invalidByte = 0
			}
			s.errorf(startOff, startLine, startCol, "illegal character %#U", cur)
			tok = token.ILLEGAL
			*tokVal = token.Value{Raw: string(cur), Pos: makeSafePos(startLine, startCol)}
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
