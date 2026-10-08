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
	file string
}

var held = struct {
	sync.Mutex
	locks map[string]*heldLock
}{locks: map[string]*heldLock{}}

func AcquireLock(root string) (release func(), err error) {
	file, err := lockPath(root)
	if err != nil {
		return nil, err
	}
	mine, err := acquireLock(file)
	if err != nil {
		return nil, err
	}
	return func() { releaseLock(mine) }, nil
}

func ReleaseHeldLocks() {
	held.Lock()
	defer held.Unlock()
	for file := range held.locks {
		removeLock(file)
	}
	clear(held.locks)
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

func acquireLock(file string) (*heldLock, error) {
	held.Lock()
	defer held.Unlock()
	if err := writeLock(file); err != nil {
		return nil, err
	}
	mine := &heldLock{file: file}
	held.locks[file] = mine
	return mine, nil
}

func writeLock(file string) error {
	lock, err := os.OpenFile(file, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o666)
	switch {
	case errors.Is(err, fs.ErrExist), err != nil && fsx.IsDir(file):
		return errHeld(readLockHolder(file))
	case err != nil:
		return errLockNotWritten(err)
	}
	_, err = lock.WriteString(strconv.Itoa(os.Getpid()))
	if closed := lock.Close(); err == nil {
		err = closed
	}
	if err != nil {
		removeLock(file)
		return errLockNotWritten(err)
	}
	return nil
}

func readLockHolder(file string) string {
	data, err := os.ReadFile(file)
	if holder := fsx.TrimASCIISpace(string(data)); err == nil && holder != "" {
		return holder
	}
	return "unknown"
}

func releaseLock(mine *heldLock) {
	held.Lock()
	defer held.Unlock()
	if held.locks[mine.file] == mine {
		delete(held.locks, mine.file)
		removeLock(mine.file)
	}
}

func removeLock(file string) {
	_ = os.Remove(file)
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
