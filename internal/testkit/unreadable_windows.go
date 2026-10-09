package testkit

import (
	"os"
	"syscall"
	"testing"
	"unsafe"
)

const (
	lockFailImmediately = 0x1
	lockExclusive       = 0x2
)

var (
	kernel32     = syscall.NewLazyDLL("kernel32.dll")
	lockFileEx   = kernel32.NewProc("LockFileEx")
	unlockFileEx = kernel32.NewProc("UnlockFileEx")
)

func MakeUnreadable(t testing.TB, path string) {
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

func MakeUnwritable(t testing.TB, path string) {
	t.Helper()
	name, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	handle, err := syscall.CreateFile(name, syscall.GENERIC_READ, syscall.FILE_SHARE_READ, nil, syscall.OPEN_EXISTING,
		syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { syscall.CloseHandle(handle) })
	if file, err := os.OpenFile(path, os.O_WRONLY, 0); err == nil {
		file.Close()
		t.Skip("this system let a second program write a file that is held for reading only; " +
			"the case is covered on the other system's run")
	}
}

func lockWholeFile(t testing.TB, handle syscall.Handle) {
	t.Helper()
	const wholeFile = 0xFFFFFFFF
	var overlapped syscall.Overlapped
	locked, _, err := lockFileEx.Call(uintptr(handle), lockExclusive|lockFailImmediately, 0, wholeFile, wholeFile,
		uintptr(unsafe.Pointer(&overlapped)))
	if locked == 0 {
		t.Fatalf("locking the file: %v", err)
	}
	t.Cleanup(func() {
		var overlapped syscall.Overlapped
		unlockFileEx.Call(uintptr(handle), 0, wholeFile, wholeFile, uintptr(unsafe.Pointer(&overlapped)))
	})
}
