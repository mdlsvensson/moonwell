package settings

import (
	"syscall"
	"testing"
)

// makeUnreadable opens the file at path the way a program does that shares it with nobody, so that reading it
// fails. The file is let go when the test ends.
func makeUnreadable(t *testing.T, path string) {
	t.Helper()
	name, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	handle, err := syscall.CreateFile(name, syscall.GENERIC_READ, 0, nil, syscall.OPEN_EXISTING,
		syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { syscall.CloseHandle(handle) })
}
