// Package mpq writes MPQ archives, the format of a packed Warcraft III map.
package mpq

// HashType selects one of the hashes MPQ takes of a file name.
type HashType uint32

const (
	TableOffset HashType = iota
	NameA
	NameB
	FileKey
)

var cryptTable = func() (table [0x500]uint32) {
	seed := uint32(0x00100001)
	for index1 := range 0x100 {
		for i, index2 := 0, index1; i < 5; i, index2 = i+1, index2+0x100 {
			seed = (seed*125 + 3) % 0x2aaaab
			high := (seed & 0xffff) << 0x10
			seed = (seed*125 + 3) % 0x2aaaab
			table[index2] = high | (seed & 0xffff)
		}
	}
	return table
}()

// HashString is the MPQ string hash; names are upper-cased (ASCII only), as the game does.
func HashString(s string, hashType HashType) uint32 {
	seed1, seed2 := uint32(0x7fed7fed), uint32(0xeeeeeeee)
	for i := 0; i < len(s); i++ {
		c := uint32(s[i])
		if c >= 'a' && c <= 'z' {
			c -= 0x20
		}
		seed1 = cryptTable[uint32(hashType)<<8+c] ^ (seed1 + seed2)
		seed2 = c + seed1 + seed2 + seed2<<5 + 3
	}
	return seed1
}

// The keys the hash table and the block table are encrypted with.
var (
	HashTableKey  = HashString("(hash table)", FileKey)
	BlockTableKey = HashString("(block table)", FileKey)
)

func nextKey(key uint32) uint32 {
	return (^key<<0x15 + 0x11111111) | key>>0x0b
}

// EncryptBlock encrypts words in place.
func EncryptBlock(words []uint32, key uint32) {
	seed := uint32(0xeeeeeeee)
	for i, plain := range words {
		seed += cryptTable[0x400+key&0xff]
		words[i] = plain ^ (key + seed)
		key = nextKey(key)
		seed = plain + seed + seed<<5 + 3
	}
}

// DecryptBlock decrypts words in place.
func DecryptBlock(words []uint32, key uint32) {
	seed := uint32(0xeeeeeeee)
	for i, cipher := range words {
		seed += cryptTable[0x400+key&0xff]
		plain := cipher ^ (key + seed)
		words[i] = plain
		key = nextKey(key)
		seed = plain + seed + seed<<5 + 3
	}
}
