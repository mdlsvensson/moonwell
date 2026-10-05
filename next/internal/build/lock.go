package build

import (
	"errors"
	"io/fs"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/mdlsvensson/moonwell/next/internal/diag"
	"github.com/mdlsvensson/moonwell/next/internal/fsx"
)

const (
	// distDir is the folder of a project that Moonwell writes what it builds into, from the project folder.
	distDir = "dist"
	// lockName is the build lock, from the project folder.
	lockName = distDir + "/.lock"
)

// held is the lock files this process made and has not given back, each by its place on disk.
//
// It is the one piece of state this package keeps between calls. A second Ctrl+C ends the program from outside
// the command that runs: what handles it knows no project and has no release function, and must still leave no
// lock behind, which the next build would take for a build that is running (ReleaseHeld). It runs beside the
// command that holds the lock, so the mutex guards every look at the list.
var held = struct {
	sync.Mutex
	files map[string]bool
}{files: map[string]bool{}}

// Acquire takes the build lock of the project at root, dist/.lock, which holds this process's id, so that a
// second build in the project fails at once. release gives it back and may be called more than once.
func Acquire(root string) (release func(), err error) {
	file, err := lockPlace(root)
	if err != nil {
		return nil, err
	}
	if err := writeLock(file); err != nil {
		return nil, err
	}
	hold(file)
	return func() { giveBack(file) }, nil
}

// ReleaseHeld removes every lock this process holds: for leaving on a second Ctrl+C without a stale lock.
func ReleaseHeld() {
	held.Lock()
	defer held.Unlock()
	for file := range held.files {
		removeLock(file)
	}
	clear(held.files)
}

// lockPlace is where the lock of the project at root is on disk, in a dist folder that is there: it is made when
// the project has none. A link at dist/ or in the lock's place is refused, so that no lock is written outside
// the project.
func lockPlace(root string) (string, error) {
	dir, err := fsx.Inside(root, distDir)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o777); err != nil {
		return "", errNoDist(err)
	}
	return fsx.Inside(root, lockName)
}

// writeLock makes the lock file with this process's id in it. A file that is there is another build's lock: it
// is left as it is, and the build that holds it is named. A lock that could not be written whole is removed.
func writeLock(file string) error {
	lock, err := os.OpenFile(file, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o666)
	switch {
	case errors.Is(err, fs.ErrExist):
		return errHeld(holderOf(file))
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

// holderOf is the process id a lock file holds, as its text without the white space around it, which is ASCII's;
// "unknown" for a lock that holds none or cannot be read.
func holderOf(file string) string {
	data, err := os.ReadFile(file)
	if holder := strings.Trim(string(data), " \t\n\v\f\r"); err == nil && holder != "" {
		return holder
	}
	return "unknown"
}

// hold notes a lock file as one this process holds.
func hold(file string) {
	held.Lock()
	defer held.Unlock()
	held.files[file] = true
}

// giveBack removes a lock file this process holds, and forgets it. A lock that was given back already is left
// alone, whoever holds a file in its place by now: another build may have taken the lock since.
func giveBack(file string) {
	held.Lock()
	defer held.Unlock()
	if held.files[file] {
		delete(held.files, file)
		removeLock(file)
	}
}

// removeLock removes a lock file. A failure is passed over: what gives a lock back is a deferred call or the end
// of the program, with nobody to tell, and the lock that stays is named, with how to remove it, by the next
// build.
func removeLock(file string) {
	_ = os.Remove(file)
}

// ---- errors ----

func errHeld(holder string) error {
	return &diag.Error{
		Msg:  "Another Moonwell build is running in this project.",
		File: lockName,
		Hint: "Wait for it to finish. If process " + holder + " is not running, delete " + lockName + ".",
	}
}

// distHint ends a failure to make dist/ or to write the lock in it.
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
