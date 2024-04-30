package token

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTokenString(t *testing.T) {
	for tok := Token(0); tok <= maxToken; tok++ {
		if tok.String() == "" {
			t.Errorf("missing string representation of token %d", tok)
		}
	}
}

func TestLookupKw(t *testing.T) {
	for tok := Token(0); tok <= maxToken; tok++ {
		expect := tok >= kwStart && tok <= kwEnd
		val := LookupKw(tok.GoString())
		if expect {
			require.Equal(t, tok, val)
		} else {
			require.Equal(t, IDENT, val)
		}
	}
}

func TestLookupPunct(t *testing.T) {
	for tok := Token(0); tok <= maxToken; tok++ {
		expect := tok >= punctStart && tok <= punctEnd
		val := LookupPunct(tok.String())
		if expect {
			require.Equal(t, tok, val)
		} else {
			require.Equal(t, ILLEGAL, val)
		}
	}
}

func TestIsAugBinop(t *testing.T) {
	for tok := Token(0); tok <= maxToken; tok++ {
		expect := tok >= augopStart && tok <= augopEnd
		got := tok.IsAugBinop()
		require.Equal(t, expect, got)
	}
}

func TestIsBinop(t *testing.T) {
	for tok := Token(0); tok <= maxToken; tok++ {
		maybe := (tok >= punctStart && tok <= punctEnd && !tok.IsAugBinop()) || tok == AND || tok == OR
		got := tok.IsBinop()
		if !maybe {
			require.False(t, got)
		}
	}
}

func TestIsUnop(t *testing.T) {
	for tok := Token(0); tok <= maxToken; tok++ {
		maybe := (tok >= punctStart && tok <= punctEnd && !tok.IsAugBinop()) || tok == NOT || tok == TRY || tok == MUST
		got := tok.IsUnop()
		if !maybe {
			require.False(t, got)
		}
	}
}

func TestIsAtom(t *testing.T) {
	for tok := Token(0); tok <= maxToken; tok++ {
		maybe := (tok >= litStart && tok <= litEnd) || tok == NULL || tok == TRUE || tok == FALSE
		got := tok.IsAtom()
		if !maybe {
			require.False(t, got)
		}
	}
}

func TestLiteral(t *testing.T) {
	val := Value{
		Raw:    "ident",
		String: "string",
		Int:    1,
		Float:  2,
	}

	got := IDENT.Literal(val)
	require.Equal(t, val.Raw, got)
	got = STRING.Literal(val)
	require.Equal(t, `"string"`, got)
	got = COMMENT.Literal(val)
	require.Equal(t, val.String, got)
	got = INT.Literal(val)
	require.Equal(t, "1", got)
	got = INT.Literal(val)
	require.Equal(t, "1", got)
	got = FLOAT.Literal(val)
	require.Equal(t, "2", got)
	got = ILLEGAL.Literal(val)
	require.Equal(t, "", got)
}
