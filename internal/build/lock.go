package build

import (
	"errors"
	"io/fs"
	"os"
	"strconv"
	"sync"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/fsx"
)

const lockName = distDir + "/.lock"

type heldLock struct {
	fullPath string
}

var heldLocks = struct {
	sync.Mutex
	locks map[string]*heldLock
}{locks: map[string]*heldLock{}}

func AcquireLock(root string) (release func(), err error) {
	fullPath, err := lockPath(root)
	if err != nil {
		return nil, err
	}
	lock, err := acquireLock(fullPath)
	if err != nil {
		return nil, err
	}
	return func() { releaseLock(lock) }, nil
}

func ReleaseHeldLocks() {
	heldLocks.Lock()
	defer heldLocks.Unlock()
	for fullPath := range heldLocks.locks {
		removeLock(fullPath)
	}
	clear(heldLocks.locks)
}

func lockPath(root string) (string, error) {
	dir, err := outputPath(root, distDir)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o777); err != nil {
		return "", errNoDist(err)
	}
	return outputPath(root, lockName)
}

func acquireLock(fullPath string) (*heldLock, error) {
	heldLocks.Lock()
	defer heldLocks.Unlock()
	if err := writeLock(fullPath); err != nil {
		return nil, err
	}
	lock := &heldLock{fullPath: fullPath}
	heldLocks.locks[fullPath] = lock
	return lock, nil
}

func writeLock(fullPath string) error {
	out, err := os.OpenFile(fullPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o666)
	switch {
	case errors.Is(err, fs.ErrExist), err != nil && fsx.IsDir(fullPath):
		return errHeld(readLockHolder(fullPath))
	case err != nil:
		return errLockNotWritten(err)
	}
	_, err = out.WriteString(strconv.Itoa(os.Getpid()))
	if closeErr := out.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		removeLock(fullPath)
		return errLockNotWritten(err)
	}
	return nil
}

func readLockHolder(fullPath string) string {
	data, err := os.ReadFile(fullPath)
	if holder := fsx.TrimASCIISpace(string(data)); err == nil && holder != "" {
		return holder
	}
	return "unknown"
}

func releaseLock(lock *heldLock) {
	heldLocks.Lock()
	defer heldLocks.Unlock()
	if heldLocks.locks[lock.fullPath] == lock {
		delete(heldLocks.locks, lock.fullPath)
		removeLock(lock.fullPath)
	}
}

func removeLock(fullPath string) {
	_ = os.Remove(fullPath)
}

func errHeld(holder string) error {
	return &diag.Error{
		Msg:  "Another Moonwell build is running in this project.",
		File: lockName,
		Hint: "Wait for it to finish. If process " + holder + " is not running, delete " + lockName + ".",
	}
}

const distHint = "Moonwell writes dist/ itself: make sure the project folder can be written and that dist is a " +
	"folder, then try again."

func errNoDist(cause error) error {
	return &diag.Error{Msg: "Creating dist/ failed: " + fsx.Reason(cause), File: distDir, Hint: distHint, Cause: cause}
}

func errLockNotWritten(cause error) error {
	return &diag.Error{
		Msg:   "Writing " + lockName + " failed: " + fsx.Reason(cause),
		File:  lockName,
		Hint:  distHint,
		Cause: cause,
	}
}
