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

// lockName is the build lock, from the project folder.
const lockName = distDir + "/.lock"

// holding is one taking of a lock: what the list keeps for the lock's file. A release gives back the lock only
// while the list keeps the holding of its own call, so it never removes a lock that a later call took at the
// same place. A holding has a size, so that two of them are never one pointer.
type holding struct {
	file string // the lock file's place on disk
}

// held is the locks this process took and has not given back, each by the place of its file on disk.
//
// It is the one piece of state this package keeps between calls. A second Ctrl+C ends the program from outside
// the command that runs: what handles it knows no project and has no release function, and must still leave no
// lock behind, which the next build would take for a build that is running (ReleaseHeld). It runs beside the
// command that holds the lock, so the mutex guards every look at the list, and a lock file is made and removed
// only under it: the list and the files of this process's locks never differ where another goroutine looks.
var held = struct {
	sync.Mutex
	locks map[string]*holding
}{locks: map[string]*holding{}}

// Acquire takes the build lock of the project at root, dist/.lock, which holds this process's id, so that a
// second build in the project fails at once. release gives it back and may be called more than once.
func Acquire(root string) (release func(), err error) {
	file, err := lockPlace(root)
	if err != nil {
		return nil, err
	}
	mine, err := take(file)
	if err != nil {
		return nil, err
	}
	return func() { giveBack(mine) }, nil
}

// ReleaseHeld removes every lock this process holds: for leaving on a second Ctrl+C without a stale lock.
func ReleaseHeld() {
	held.Lock()
	defer held.Unlock()
	for file := range held.locks {
		removeLock(file)
	}
	clear(held.locks)
}

// lockPlace is where the lock of the project at root is on disk, in a dist folder that is there: it is made when
// the project has none. A link at dist or in the lock's place is refused, as outputAt says, before the folder is
// made: no lock is written outside the project.
func lockPlace(root string) (string, error) {
	dir, err := outputAt(root, distDir)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o777); err != nil {
		return "", errNoDist(err)
	}
	return outputAt(root, lockName)
}

// take makes the lock file and notes it as held, in one step under the list's mutex: a release of every lock
// that runs beside it removes the file or does not see it yet, and never leaves it behind unnoted.
func take(file string) (*holding, error) {
	held.Lock()
	defer held.Unlock()
	if err := writeLock(file); err != nil {
		return nil, err
	}
	mine := &holding{file: file}
	held.locks[file] = mine
	return mine, nil
}

// writeLock makes the lock file with this process's id in it. What is in the lock's place is another build's
// lock: it is left as it is, and the build that holds it is named. A lock that could not be written whole is
// removed.
//
// A folder in the lock's place is a lock that is held too, by nobody that can be named: one system says of it
// that it is there, as of a file, and another that it is a folder.
func writeLock(file string) error {
	lock, err := os.OpenFile(file, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o666)
	switch {
	case errors.Is(err, fs.ErrExist), err != nil && fsx.IsDir(file):
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
	if holder := fsx.TrimASCIISpace(string(data)); err == nil && holder != "" {
		return holder
	}
	return "unknown"
}

// giveBack removes the lock file of a holding and forgets the holding, as long as the list keeps it for the file.
// A holding that was given back, by its own release or by a release of every lock, is not in the list: the list
// then keeps nothing for the file, or the holding of a later call. A file that is there then is that call's lock,
// or another process's, and is left alone.
func giveBack(mine *holding) {
	held.Lock()
	defer held.Unlock()
	if held.locks[mine.file] == mine {
		delete(held.locks, mine.file)
		removeLock(mine.file)
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
