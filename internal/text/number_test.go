package text

import (
	"math"
	"testing"
)

func TestNumberIsJavaScriptsNumberToString(t *testing.T) {
	for _, c := range []struct {
		value float64
		want  string
	}{
		{0, "0"},
		{math.Copysign(0, -1), "0"},
		{128, "128"},
		{-896, "-896"},
		{0.5, "0.5"},
		{1.0 / 255, "0.00392156862745098"},
		{float64(float32(0.1)), "0.10000000149011612"},
		{1e21, "1e+21"},
		{1e20, "100000000000000000000"},
		{123456789012345680000, "123456789012345680000"},
		{1e-7, "1e-7"},
		{0.000001, "0.000001"},
		{1.5e-7, "1.5e-7"},
		{-2.5e-10, "-2.5e-10"},
		{1.7976931348623157e308, "1.7976931348623157e+308"},
		{5e-324, "5e-324"},
		{4294967295, "4294967295"},
		{1234.5678, "1234.5678"},
		{100, "100"},
		{0.1, "0.1"},
		{math.Inf(1), "Infinity"},
		{math.Inf(-1), "-Infinity"},
		{math.NaN(), "NaN"},
	} {
		if got := Number(c.value); got != c.want {
			t.Errorf("Number(%g) = %q, want %q", c.value, got, c.want)
		}
	}
}
