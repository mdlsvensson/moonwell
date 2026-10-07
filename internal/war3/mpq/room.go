package mpq

// This file holds how large an archive can be: the most bytes the archive Write makes of some files can take, and
// whether the format can hold that.

// MaxSize is the largest number a word of the format holds, which writes every position and size as one. So it is
// the most bytes a file of an archive can have, and the most an archive can have with what stands before it.
const MaxSize = 1<<32 - 1

// sized is a file of an archive by its name there and the number of its bytes.
type sized struct {
	name string
	size int64
}

// RoomFor reports whether the format can hold the archive Write makes of the files behind a prefix of so many
// bytes, with sectors of the size Write gives them unless its options say otherwise. Where it cannot, tooLarge is
// the name of the first file that is too large by itself, and "" where the files are too large together. The
// answer depends on the names and the sizes of the files alone.
func RoomFor(prefix int, files []File) (tooLarge string, fits bool) {
	sizes := make([]sized, len(files))
	for at, file := range files {
		sizes[at] = sized{name: file.Name, size: int64(len(file.Data))}
	}
	return roomFor(int64(prefix), sizes)
}

// roomFor is RoomFor for files that are known by their sizes.
//
// The format writes as a word: the size of each file; where each file's data starts and how long it is, counted
// from the archive's header; where the two tables start; and the archive's size, from its header to the end of
// the block table, which is the largest of them. So a file must not be larger than a word holds, and the archive
// must not be: it is counted with its prefix, the header of 512 bytes that a map of an older format has, since
// a reader finds a position by adding where the archive starts, and holds the sum in a word too. The archive
// is counted at its largest, with no sector compressed, so the answer depends on the sizes alone.
func roomFor(prefix int64, files []sized) (tooLarge string, fits bool) {
	for _, file := range files {
		if file.size > MaxSize {
			return file.name, false
		}
	}
	return "", largestArchive(prefix, files) <= MaxSize
}

// largestArchive is the most bytes an archive of the files takes, with its prefix: the bytes it takes when no
// sector of it is stored shorter than it is. That is the prefix, the header, the data of each file and of the
// list of the files, the hash table and the block table.
func largestArchive(prefix int64, files []sized) int64 {
	total := prefix + headerSize
	var list int64
	for _, file := range files {
		total += largestStored(file.size)
		list += int64(len(file.name)) + 2 // the file's line of the list: its name and "\r\n"
	}
	entries := len(files) + 1 // the files, and the list of them
	return total + largestStored(list) + int64(hashSlots(entries)+entries)*entryWords*wordSize
}

// largestStored is the most bytes the data of a file of size bytes takes in an archive: its bytes, and a word
// for where each sector starts and one for where the last ends. A file without a byte takes none.
func largestStored(size int64) int64 {
	if size == 0 {
		return 0
	}
	const sector = smallestSector << defaultSectorShift
	sectors := (size + sector - 1) / sector
	return size + (sectors+1)*wordSize
}
