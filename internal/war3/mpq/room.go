package mpq

const MaxSize = 1<<32 - 1

type sized struct {
	name string
	size int64
}

func RoomFor(prefix int, files []File) (tooLarge string, fits bool) {
	sizes := make([]sized, len(files))
	for at, file := range files {
		sizes[at] = sized{name: file.Name, size: int64(len(file.Data))}
	}
	return roomFor(int64(prefix), sizes)
}

func roomFor(prefix int64, files []sized) (tooLarge string, fits bool) {
	for _, file := range files {
		if file.size > MaxSize {
			return file.name, false
		}
	}
	return "", largestArchive(prefix, files) <= MaxSize
}

func largestArchive(prefix int64, files []sized) int64 {
	total := prefix + headerSize
	var list int64
	for _, file := range files {
		total += largestStored(file.size)
		list += int64(len(file.name)) + 2
	}
	entries := len(files) + 1
	return total + largestStored(list) + int64(hashSlots(entries)+entries)*entryWords*wordSize
}

func largestStored(size int64) int64 {
	if size == 0 {
		return 0
	}
	const sector = smallestSector << defaultSectorShift
	sectors := (size + sector - 1) / sector
	return size + (sectors+1)*wordSize
}
