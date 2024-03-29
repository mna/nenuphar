package scanner

import (
	"strconv"
	"strings"

	"github.com/mna/nenuphar/lang/token"
)

func (s *Scanner) number() (tok token.Token, base int, lit string) {
	startOff, startLine, startCol := s.off, s.line, s.col
	tok = token.ILLEGAL

	base = 10         // number base
	prefix := rune(0) // one of 0 (decimal), 'x', 'o', or 'b'
	digsep := 0       // bit 0: digit present, bit 1: '_' present
	invalid := -1     // index of invalid digit in literal, or < 0

	// integer part
	if s.cur != '.' {
		tok = token.INT
		if s.cur == '0' {
			s.advance()
			switch lower(s.cur) {
			case 'x':
				s.advance()
				base, prefix = 16, 'x'
			case 'o':
				s.advance()
				base, prefix = 8, 'o'
			case 'b':
				s.advance()
				base, prefix = 2, 'b'
			}
		}
		digsep |= s.digits(base, &invalid)
	}

	// fractional part
	if s.cur == '.' {
		tok = token.FLOAT
		if prefix == 'o' || prefix == 'b' {
			s.error(s.off, s.line, s.col, "invalid radix point in "+litname(prefix))
		}
		s.advance()
		digsep |= s.digits(base, &invalid)
	}

	if digsep&1 == 0 {
		s.error(s.off, s.line, s.col, litname(prefix)+" has no digits")
	}

	// exponent
	if e := lower(s.cur); e == 'e' || e == 'p' {
		switch {
		case e == 'e' && prefix != 0:
			s.errorf(s.off, s.line, s.col, "%q exponent requires decimal mantissa", s.cur)
		case e == 'p' && prefix != 'x':
			s.errorf(s.off, s.line, s.col, "%q exponent requires hexadecimal mantissa", s.cur)
		}
		s.advance()
		tok = token.FLOAT
		if s.cur == '+' || s.cur == '-' {
			s.advance()
		}
		ds := s.digits(10, nil)
		digsep |= ds
		if ds&1 == 0 {
			s.error(s.off, s.line, s.col, "exponent has no digits")
		}
	} else if prefix == 'x' && tok == token.FLOAT {
		s.error(s.off, s.line, s.col, "hexadecimal mantissa requires a 'p' exponent")
	}

	lit = string(s.src[startOff:s.off])
	if tok == token.INT && invalid >= 0 {
		s.errorf(invalid, startLine, startCol+invalid-startOff, "invalid digit %q in %s", lit[invalid-startOff], litname(prefix))
	}
	if digsep&2 != 0 {
		if i := invalidSep(lit); i >= 0 {
			s.error(startOff+i, startLine, startCol+i, "'_' must separate successive digits")
		}
	}
	return tok, base, lit
}

func isDecimal(rn rune) bool {
	return '0' <= rn && rn <= '9'
}

func isHexadecimal(rn rune) bool {
	return isDecimal(rn) ||
		'a' <= rn && rn <= 'f' ||
		'A' <= rn && rn <= 'F'
}

// digits accepts the sequence { digit | '_' }.
// If base <= 10, digits accepts any decimal digit but records
// the offset (relative to the source start) of a digit >= base
// in *invalid, if *invalid < 0.
// digits returns a bitset describing whether the sequence contained
// digits (bit 0 is set), or separators '_' (bit 1 is set).
func (s *Scanner) digits(base int, invalid *int) (digsep int) {
	if base <= 10 {
		max := rune('0' + base)
		for isDecimal(s.cur) || s.cur == '_' {
			ds := 1
			if s.cur == '_' {
				ds = 2
			} else if s.cur >= max && *invalid < 0 {
				*invalid = s.off
			}
			digsep |= ds
			s.advance()
		}
	} else {
		for isHexadecimal(s.cur) || s.cur == '_' {
			ds := 1
			if s.cur == '_' {
				ds = 2
			}
			digsep |= ds
			s.advance()
		}
	}
	return
}

// invalidSep returns the index of the first invalid separator in x, or -1.
func invalidSep(x string) int {
	x1 := ' ' // prefix char, we only care if it's 'x'
	d := '.'  // digit, one of '_', '0' (a digit), or '.' (anything else)
	i := 0

	// a prefix counts as a digit
	if len(x) >= 2 && x[0] == '0' {
		x1 = lower(rune(x[1]))
		if x1 == 'x' || x1 == 'o' || x1 == 'b' {
			d = '0'
			i = 2
		}
	}

	// mantissa and exponent
	for ; i < len(x); i++ {
		p := d // previous digit
		d = rune(x[i])
		switch {
		case d == '_':
			if p != '0' {
				return i
			}
		case isDecimal(d) || x1 == 'x' && isHexadecimal(d):
			d = '0'
		default:
			if p == '_' {
				return i - 1
			}
			d = '.'
		}
	}
	if d == '_' {
		return len(x) - 1
	}

	return -1
}

func litname(prefix rune) string {
	switch prefix {
	case 'x':
		return "hexadecimal literal"
	case 'o':
		return "octal literal"
	case 'b':
		return "binary literal"
	}
	return "decimal literal"
}

func lower(ch rune) rune {
	return ('a' - 'A') | ch // returns lower-case ch iff ch is ASCII letter
}

func numberToInt(lit string, base int) (int64, error) {
	if base != 10 {
		// skip the 0x/0o/0b prefix
		lit = lit[2:]
	}
	// underscores and prefix must be removed when a base is provided
	return strconv.ParseInt(strings.ReplaceAll(lit, "_", ""), base, 64)
}

func numberToFloat(lit string) (float64, error) {
	// underscores and 0x prefix are fine for ParseFloat.
	return strconv.ParseFloat(lit, 64)
}
