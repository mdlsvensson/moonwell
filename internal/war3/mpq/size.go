package mpq

const MaxSize = 1<<32 - 1

type fileSize struct {
	name string
	size int64
}

func CheckFits(prefix int, files []File) (tooLarge string, fits bool) {
	sizes := make([]fileSize, len(files))
	for i, file := range files {
		sizes[i] = fileSize{name: file.Name, size: int64(len(file.Data))}
	}
	return checkFits(int64(prefix), sizes)
}

func checkFits(prefix int64, files []fileSize) (tooLarge string, fits bool) {
	for _, file := range files {
		if file.size > MaxSize {
			return file.name, false
		}
	}
	return "", maxArchiveSize(prefix, files) <= MaxSize
}

func maxArchiveSize(prefix int64, files []fileSize) int64 {
	total := prefix + headerSize
	var list int64
	for _, file := range files {
		total += maxStoredSize(file.size)
		list += int64(len(file.name) + len(listfileNewline))
	}
	entries := len(files) + 1
	return total + maxStoredSize(list) + int64(hashSlots(entries)+entries)*entryWords*wordSize
}

func maxStoredSize(size int64) int64 {
	if size == 0 {
		return 0
	}
	const sector = smallestSector << defaultSectorShift
	sectors := (size + sector - 1) / sector
	return size + (sectors+1)*wordSize
}
