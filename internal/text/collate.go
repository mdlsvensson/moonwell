package text

import "slices"

// asciiOrder lists the ASCII characters that have a place of their own in the order JavaScript's localeCompare
// gives (the Unicode root collation): white space, punctuation, symbols, digits, then letters, where a letter's
// two cases share a place.
const asciiOrder = "\t\n\v\f\r _-,;:!?.'\"()[]{}@*/\\&#%`^+<=>|~$0123456789abcdefghijklmnopqrstuvwxyz"

// asciiPlace is each ASCII character's place in asciiOrder, counted from 1; 0 for the control characters, which
// the comparison skips.
var asciiPlace = func() (places [128]int) {
	for i := 0; i < len(asciiOrder); i++ {
		places[asciiOrder[i]] = i + 1
	}
	for c := 'A'; c <= 'Z'; c++ {
		places[c] = places[c+('a'-'A')]
	}
	return places
}()

// collationKeys gives the two levels a string is compared by: each character's place, and whether it is a capital.
func collationKeys(s string) (places, capitals []int) {
	for _, r := range s {
		switch {
		case r >= 128:
			// Past ASCII the places are not the root collation's: every such character follows the letters, by code point.
			places, capitals = append(places, len(asciiOrder)+int(r)), append(capitals, 0)
		case asciiPlace[r] != 0:
			capital := 0
			if r >= 'A' && r <= 'Z' {
				capital = 1
			}
			places, capitals = append(places, asciiPlace[r]), append(capitals, capital)
		}
	}
	return places, capitals
}

// LocaleCompare orders two strings as JavaScript's a.localeCompare(b) does when both are ASCII: by each character's
// place in the Unicode root collation first (punctuation before digits before letters, "a" and "A" alike), and by
// case, lower first, only between strings that are otherwise the same. Characters past ASCII sort after the
// letters by code point, which is not what JavaScript does with them.
func LocaleCompare(a, b string) int {
	placesA, capitalsA := collationKeys(a)
	placesB, capitalsB := collationKeys(b)
	if order := slices.Compare(placesA, placesB); order != 0 {
		return order
	}
	return slices.Compare(capitalsA, capitalsB)
}
