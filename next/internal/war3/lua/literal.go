package lua

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

var (
	hexInteger = regexp.MustCompile(`^0[xX][0-9a-fA-F]+$`)
	decimal    = regexp.MustCompile(`^(?:[0-9]+(?:\.[0-9]*)?|\.[0-9]+)(?:[eE][+-]?[0-9]+)?$`)
)

// LiteralNumber returns the value of tokens when they are one literal decimal number or hexadecimal integer,
// optionally negated.
func LiteralNumber(tokens []Token) (float64, bool) {
	negative := len(tokens) == 2 && tokens[0].is("-")
	if negative {
		tokens = tokens[1:]
	}
	if len(tokens) != 1 || tokens[0].Kind != NumberToken {
		return 0, false
	}
	value, ok := numeralValue(tokens[0].Raw)
	if ok && negative {
		value = -value
	}
	return value, ok
}

// numeralValue is the value of a decimal numeral or a hexadecimal integer. A hexadecimal numeral with a fraction or
// an exponent has none here, nor has a numeral too large for a float.
func numeralValue(raw string) (float64, bool) {
	switch {
	case hexInteger.MatchString(raw):
		// With a binary exponent the integer is a float as strconv reads one, rounded to the nearest at any length.
		raw += "p0"
	case !decimal.MatchString(raw):
		return 0, false
	}
	value, _ := strconv.ParseFloat(raw, 64) // out of range gives an infinity, refused below
	if math.IsInf(value, 0) {
		return 0, false
	}
	return value, true
}

// PlayerID returns n when tokens are `Player(n)` with a literal whole number.
func PlayerID(tokens []Token) (int, bool) {
	last := len(tokens) - 1
	if len(tokens) < 3 || tokens[0].Kind != NameToken || tokens[0].Raw != "Player" ||
		!tokens[1].is("(") || !tokens[last].is(")") {
		return 0, false
	}
	value, ok := LiteralNumber(tokens[2:last])
	if !ok || value != math.Trunc(value) {
		return 0, false
	}
	return int(value), true
}

// Quote writes s as a Lua string literal. Control characters are three-digit decimal escapes, so that a digit after
// one stays apart from it.
func Quote(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch {
		case r == '\\' || r == '"':
			b.WriteByte('\\')
			b.WriteRune(r)
		case r < 32 || r == 127:
			fmt.Fprintf(&b, `\%03d`, r)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

// Number writes v as a Lua number literal in plain decimal, with the fewest digits that read back as v. v is
// finite.
func Number(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}
