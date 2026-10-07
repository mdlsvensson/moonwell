package testkit

import "syscall"

// shortSpelling is the folder as Windows writes it with its short names, such as C:\Users\LONGNA~1 for a name
// of more than eight letters; "" for a folder that is not there or of a volume without short names.
func shortSpelling(folder string) string {
	path, err := syscall.UTF16PtrFromString(folder)
	if err != nil {
		return ""
	}
	buffer := make([]uint16, syscall.MAX_PATH)
	for {
		length, err := syscall.GetShortPathName(path, &buffer[0], uint32(len(buffer)))
		if err != nil || length == 0 {
			return ""
		}
		if int(length) <= len(buffer) {
			return syscall.UTF16ToString(buffer[:length])
		}
		buffer = make([]uint16, length)
	}
}
