package assets

import (
	"os"
	"syscall"
	"testing"
	"unsafe"
)

// The flags of LockFileEx: the lock is exclusive, and the call fails where it would wait.
const (
	lockFailImmediately = 0x1
	lockExclusive       = 0x2
)

var (
	kernel32     = syscall.NewLazyDLL("kernel32.dll")
	lockFileEx   = kernel32.NewProc("LockFileEx")
	unlockFileEx = kernel32.NewProc("UnlockFileEx")
)

// makeUnreadable holds the file at path the way a program does that lets nobody else read it: it opens the file
// and shares it with nobody, and locks every byte of it for itself through that handle. Either one refuses a
// read by another handle. The file is let go when the test ends.
//
// The helper then tries the read itself. A system on which the file can still be read cannot run the test, which
// is skipped there: what the test is about is the same on every system, and is run on the others.
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
	lockWholeFile(t, handle)
	if _, err := os.ReadFile(path); err == nil {
		t.Skip("this system let a second program read a file held with no sharing and an exclusive lock; " +
			"the case is covered on the other system's run")
	}
}

// lockWholeFile takes an exclusive lock on every byte the file has or could have, through the handle, until the
// test ends. The lock starts at the offset an Overlapped names, which is the start of the file for a zero one.
func lockWholeFile(t *testing.T, handle syscall.Handle) {
	t.Helper()
	const everyByte = 0xFFFFFFFF // the low and the high half of the number of bytes
	var fromTheStart syscall.Overlapped
	locked, _, err := lockFileEx.Call(uintptr(handle), lockExclusive|lockFailImmediately, 0, everyByte, everyByte,
		uintptr(unsafe.Pointer(&fromTheStart)))
	if locked == 0 {
		t.Fatalf("locking the file: %v", err)
	}
	t.Cleanup(func() {
		var fromTheStart syscall.Overlapped
		unlockFileEx.Call(uintptr(handle), 0, everyByte, everyByte, uintptr(unsafe.Pointer(&fromTheStart)))
	})
}
