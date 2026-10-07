package objmod

// ID is a four-character id as the files store it: an object, a base object or a field.
type ID [4]byte

// ParseID returns the id that s writes out: four characters, each at most U+00FF and so one byte of the id. It
// reports false for any other text.
func ParseID(s string) (ID, bool) {
	var id ID
	count := 0
	for _, character := range s {
		if count == len(id) || character > 0xFF {
			return ID{}, false
		}
		id[count] = byte(character)
		count++
	}
	if count != len(id) {
		return ID{}, false
	}
	return id, true
}

// String writes each byte of the id as the character with that number, U+0000 to U+00FF, so that an id of letters
// and digits reads as it does in World Editor. ParseID reads the text back.
func (id ID) String() string {
	return string([]rune{rune(id[0]), rune(id[1]), rune(id[2]), rune(id[3])})
}
