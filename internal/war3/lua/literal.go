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

func numeralValue(raw string) (float64, bool) {
	switch {
	case hexInteger.MatchString(raw):
		raw += "p0"
	case !decimal.MatchString(raw):
		return 0, false
	}
	value, _ := strconv.ParseFloat(raw, 64)
	if math.IsInf(value, 0) {
		return 0, false
	}
	return value, true
}

func PlayerID(tokens []Token) (int, bool) {
	last := len(tokens) - 1
	if len(tokens) < 3 || tokens[0].Kind != NameToken || tokens[0].Raw != "Player" ||
		!tokens[1].is("(") || !tokens[last].is(")") {
		return 0, false
	}
	value, ok := LiteralNumber(tokens[2:last])
	if !ok || value != math.Trunc(value) || math.Abs(value) >= 1<<63 {
		return 0, false
	}
	return int(value), true
}

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

func Number(v float64) string {
	if v == 0 {
		return "0"
	}
	return strconv.FormatFloat(v, 'f', -1, 64)
}
