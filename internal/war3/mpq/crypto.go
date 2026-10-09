package mpq

type HashType uint32

const (
	TableOffset HashType = iota
	NameA
	NameB
	FileKey
)

const cipherKeys HashType = 4

const (
	hashTypeCount  = 5
	byteValueCount = 1 << 8
)

var cryptTable = buildCryptTable()

func buildCryptTable() (table [hashTypeCount * byteValueCount]uint32) {
	seed := uint32(0x00100001)
	step := func() uint32 {
		seed = (seed*125 + 3) % 0x2AAAAB
		return seed & 0xFFFF
	}
	for b := range byteValueCount {
		for hashType := range hashTypeCount {
			high := step()
			table[hashType<<8+b] = high<<16 | step()
		}
	}
	return table
}

func toUpperASCII(c byte) byte {
	if 'a' <= c && c <= 'z' {
		return c - 'a' + 'A'
	}
	return c
}

func HashString(s string, hashType HashType) uint32 {
	seed1, seed2 := uint32(0x7FED7FED), uint32(0xEEEEEEEE)
	for i := range len(s) {
		c := uint32(toUpperASCII(s[i]))
		seed1 = cryptTable[uint32(hashType)<<8+c] ^ (seed1 + seed2)
		seed2 = c + seed1 + seed2 + seed2<<5 + 3
	}
	return seed1
}

var (
	HashTableKey  = HashString("(hash table)", FileKey)
	BlockTableKey = HashString("(block table)", FileKey)
)

type keystream struct {
	key, seed uint32
}

func newKeystream(key uint32) keystream {
	return keystream{key: key, seed: 0xEEEEEEEE}
}

func (k *keystream) next() uint32 {
	k.seed += cryptTable[uint32(cipherKeys)<<8+(k.key&0xFF)]
	return k.key + k.seed
}

func (k *keystream) absorb(plain uint32) {
	k.key = (^k.key<<21 + 0x11111111) | k.key>>11
	k.seed = plain + k.seed + k.seed<<5 + 3
}

func EncryptBlock(words []uint32, key uint32) {
	stream := newKeystream(key)
	for i, plain := range words {
		words[i] = plain ^ stream.next()
		stream.absorb(plain)
	}
}

func DecryptBlock(words []uint32, key uint32) {
	stream := newKeystream(key)
	for i, cipher := range words {
		plain := cipher ^ stream.next()
		words[i] = plain
		stream.absorb(plain)
	}
}
