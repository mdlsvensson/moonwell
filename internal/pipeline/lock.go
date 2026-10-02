package pipeline

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/text"
)

// held are the lock files this process created and has not released yet.
var held = struct {
	sync.Mutex
	paths map[string]bool
}{paths: map[string]bool{}}

// AcquireLock takes <distDir>/.lock, which records this process's id, so that a second build in the project fails
// at once. release gives it back and may be called more than once.
func AcquireLock(distDir string) (release func(), err error) {
	if err := os.MkdirAll(distDir, 0o777); err != nil {
		return nil, err
	}
	lockPath := filepath.Join(distDir, ".lock")
	file, err := os.OpenFile(lockPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o666)
	if errors.Is(err, fs.ErrExist) {
		holder := "unknown"
		if data, readErr := os.ReadFile(lockPath); readErr == nil && text.Trim(text.Lossy(data)) != "" {
			holder = text.Trim(text.Lossy(data))
		}
		return nil, &diag.Error{
			Msg:  "Another Moonwell build is running in this project.",
			File: lockPath,
			Hint: "Wait for it to finish. If process " + holder + " is not running, delete " + lockPath + ".",
		}
	}
	if err != nil {
		return nil, err
	}
	_, err = file.WriteString(strconv.Itoa(os.Getpid()))
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		os.Remove(lockPath)
		return nil, err
	}
	held.Lock()
	held.paths[lockPath] = true
	held.Unlock()
	return func() {
		held.Lock()
		defer held.Unlock()
		if held.paths[lockPath] {
			delete(held.paths, lockPath)
			os.Remove(lockPath)
		}
	}, nil
}

// ReleaseHeldLocks removes every lock this process holds, for leaving on Ctrl+C without a stale lock.
func ReleaseHeldLocks() {
	held.Lock()
	defer held.Unlock()
	for lockPath := range held.paths {
		os.Remove(lockPath)
	}
	clear(held.paths)
}
