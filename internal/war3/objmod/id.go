package objmod

type ID [4]byte

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

func (id ID) String() string {
	return string([]rune{rune(id[0]), rune(id[1]), rune(id[2]), rune(id[3])})
}
