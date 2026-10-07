// Package mpq writes MPQ format-1 archives, the format of a packed map. It takes named byte slices and returns
// the archive's bytes. It does not read folders.
//
// Write takes the files and returns the archive; RoomFor says whether the format can hold an archive of them;
// HM3WHeader returns the header that older maps carry before it. HashString, EncryptBlock, DecryptBlock and the
// two table keys are the format's hash and cipher, which a reader of an archive needs too.
//
// The package must not know where the files come from, what a map is made of, or which maps carry the header.
package mpq

// HashType selects one of the hashes the format takes of a file name.
type HashType uint32

const (
	// TableOffset is the hash that says where in the hash table the search for a name starts.
	TableOffset HashType = iota
	// NameA and NameB are the two hashes that a slot of the hash table holds in place of the name.
	NameA
	NameB
	// FileKey is the hash that gives the key something is encrypted with.
	FileKey
)

// cipherKeys is the hash type whose numbers EncryptBlock and DecryptBlock mix into their key.
const cipherKeys HashType = 4

// numbers are the format's 1280 fixed numbers: 256 for each of the four hash types and 256 for the cipher. The
// number of byte b for type t is at t<<8 + b.
var numbers = fixedNumbers()

// fixedNumbers makes the numbers from the format's seed. Each number is two steps of the seed: the low 16 bits of
// the first step are its high half, those of the second step its low half. The types take turns, so that the five
// numbers of one byte come from ten steps in a row.
func fixedNumbers() (table [5 << 8]uint32) {
	seed := uint32(0x00100001)
	step := func() uint32 {
		seed = (seed*125 + 3) % 0x2AAAAB
		return seed & 0xFFFF
	}
	for b := range 1 << 8 {
		for hashType := range 5 {
			high := step()
			table[hashType<<8+b] = high<<16 | step()
		}
	}
	return table
}

// upper is c as the game hashes it: an ASCII small letter becomes its capital and every other byte stays.
func upper(c byte) byte {
	if 'a' <= c && c <= 'z' {
		return c - 'a' + 'A'
	}
	return c
}

// HashString is the format's hash of a name. Names are upper-cased, ASCII only, as the game does: a letter that is
// not ASCII hashes as the bytes it is written with, so only names that differ in the case of ASCII letters have
// the same hash.
func HashString(s string, hashType HashType) uint32 {
	seed1, seed2 := uint32(0x7FED7FED), uint32(0xEEEEEEEE)
	for i := range len(s) {
		c := uint32(upper(s[i]))
		seed1 = numbers[uint32(hashType)<<8+c] ^ (seed1 + seed2)
		seed2 = c + seed1 + seed2 + seed2<<5 + 3
	}
	return seed1
}

// HashTableKey and BlockTableKey are the keys an archive's two tables are encrypted with: the file keys of the
// names the format gives the tables.
var (
	HashTableKey  = HashString("(hash table)", FileKey)
	BlockTableKey = HashString("(block table)", FileKey)
)

// keystream gives the numbers that the words of one block are combined with, one after another. Each number
// depends on the key and on every plain word before it.
type keystream struct {
	key, seed uint32
}

func newKeystream(key uint32) keystream {
	return keystream{key: key, seed: 0xEEEEEEEE}
}

// next is the number for the next word.
func (k *keystream) next() uint32 {
	k.seed += numbers[uint32(cipherKeys)<<8+(k.key&0xFF)]
	return k.key + k.seed
}

// took moves on past a word, given what the word is in plain.
func (k *keystream) took(plain uint32) {
	k.key = (^k.key<<21 + 0x11111111) | k.key>>11
	k.seed = plain + k.seed + k.seed<<5 + 3
}

// EncryptBlock encrypts words in place with key.
func EncryptBlock(words []uint32, key uint32) {
	stream := newKeystream(key)
	for i, plain := range words {
		words[i] = plain ^ stream.next()
		stream.took(plain)
	}
}

// DecryptBlock decrypts words in place with the key they were encrypted with.
func DecryptBlock(words []uint32, key uint32) {
	stream := newKeystream(key)
	for i, cipher := range words {
		plain := cipher ^ stream.next()
		words[i] = plain
		stream.took(plain)
	}
}
