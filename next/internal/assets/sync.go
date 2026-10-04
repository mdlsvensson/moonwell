package assets

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"strings"

	"github.com/mdlsvensson/moonwell/next/internal/diag"
	"github.com/mdlsvensson/moonwell/next/internal/fsx"
	"github.com/mdlsvensson/moonwell/next/internal/mapdir"
)

// Sync writes a plan's changes into the map folder itself and the ownership state into stateFile. When a write
// fails or ctx is cancelled it puts back every file it changed.
//
// folder is the folder the plan was made from. What Plan read through it is what each file is checked against
// before it is replaced or removed, so a file that changed after the plan stops the sync and is not written
// over; another folder has no such record, and is refused as the caller's bug.
//
// The state file is written last, so that it never lists a file that was not written. A state that owns nothing
// has no file: one that is there is removed. The folder of the state file is made when it is needed. Folders
// made for new files stay when the files are taken out again.
func Sync(ctx context.Context, folder *mapdir.Folder, result *Result, stateFile string) error {
	if result.planned != folder {
		return errOtherFolder()
	}
	state, err := stateChange(stateFile, result.State)
	if err != nil {
		return err
	}
	var journal fsx.Journal
	failure := write(ctx, folder.With(result.Changes), state, &journal)
	if failure == nil {
		return nil
	}
	touched := journal.Len()
	return undone(failure, touched, unrestored(folder, journal.Undo()))
}

// stateWrite is what a sync does to the state file: it writes bytes, or removes the file.
type stateWrite struct {
	file   string
	bytes  []byte
	remove bool
}

// stateChange is what the state file needs to hold the state, or nil when it is as it must be: the file of a
// state that owns nothing is removed, and any other is written unless it holds the same bytes.
func stateChange(file string, state State) (*stateWrite, error) {
	held, found, err := readIfThere(file)
	if err != nil {
		return nil, err
	}
	if len(state.Files) == 0 {
		if !found {
			return nil, nil
		}
		return &stateWrite{file: file, remove: true}, nil
	}
	written := state.Bytes()
	if found && bytes.Equal(held, written) {
		return nil, nil
	}
	return &stateWrite{file: file, bytes: written}, nil
}

// write makes the changes of the view in the map folder and then the change of the state file, all through
// journal. It stops at the first that fails, and between two of them once ctx is cancelled.
func write(ctx context.Context, view *mapdir.Folder, state *stateWrite, journal *fsx.Journal) error {
	if err := view.ApplyInPlace(ctx, journal); err != nil || state == nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return state.apply(journal)
}

// apply writes or removes the state file through journal.
func (s *stateWrite) apply(journal *fsx.Journal) error {
	var err error
	if s.remove {
		err = journal.Remove(s.file)
	} else {
		err = journal.Write(s.file, s.bytes)
	}
	if err != nil {
		return errStateNotWritten(s.file, err)
	}
	return nil
}

// unrestored lists the files an undo could not put back, each with the system's reason: a file of the map by
// the folder's label, the state file by its path.
func unrestored(folder *mapdir.Folder, files []fsx.Unrestored) []string {
	var listed []string
	for _, file := range files {
		listed = append(listed, named(folder, file.Path)+" ("+fsx.Reason(file.Err)+")")
	}
	return listed
}

// named is how errors name the file at path: one inside the map folder by the folder's label, any other by its
// path.
func named(folder *mapdir.Folder, path string) string {
	dir, err := filepath.Abs(folder.Dir())
	if err != nil {
		return path
	}
	below, err := filepath.Rel(dir, path)
	if err != nil || !filepath.IsLocal(below) {
		return path
	}
	return folder.Label(filepath.ToSlash(below))
}

// undone is what a sync that stopped tells its caller, after the undo: touched is how many files it had written
// or removed when it stopped, and unrestored the files the undo could not put back.
func undone(failure error, touched int, unrestored []string) error {
	var expected *diag.Error
	isExpected := errors.As(failure, &expected)
	switch {
	case len(unrestored) > 0:
		return errNotRestored(failure, unrestored)
	case interrupted(failure) && touched == 0:
		return errInterruptedBeforeWriting()
	case interrupted(failure):
		return errInterruptedAndUndone()
	case isExpected && expected.Cause != nil:
		return errNotWritten(expected)
	}
	// A file that changed after the plan, which names itself; or a plan that cannot be written, which is a bug.
	return failure
}

// interrupted reports whether a sync stopped because its context was cancelled.
func interrupted(failure error) bool {
	return errors.Is(failure, context.Canceled) || errors.Is(failure, context.DeadlineExceeded)
}

// reasonOf is why a sync stopped, in the words that go into another message: the system's reason for a write
// that failed, and the message of a failure that has no cause.
func reasonOf(failure error) string {
	var expected *diag.Error
	switch {
	case interrupted(failure):
		return "interrupted"
	case !errors.As(failure, &expected):
		return fsx.Reason(failure)
	case expected.Cause != nil:
		return fsx.Reason(expected.Cause)
	}
	return expected.Msg
}

// ---- errors ----

// errInterruptedBeforeWriting, for a sync that is stopped before its first write, is with Plan in plan.go.

// errOtherFolder is not a diag error: only a caller's bug gives Sync another folder than the plan's.
func errOtherFolder() error {
	return errors.New("Cannot sync the assets: the folder is not the folder the plan was made from.")
}

func errInterruptedAndUndone() error {
	return &diag.Error{Msg: "Interrupted; every change was undone."}
}

// errStateNotWritten is the failure of the state file's write, in the shape of mapdir's failure of a map file's:
// undone words both as one.
func errStateNotWritten(file string, cause error) error {
	return &diag.Error{
		Msg:   "Writing the asset ownership state failed: " + fsx.Reason(cause),
		File:  file,
		Cause: cause,
		Hint:  "Make sure the file and its folder can be written and no other program has the file open, then retry.",
	}
}

// errNotWritten is the failure of a write, of a map file or of the state file, after every change was undone.
func errNotWritten(failed *diag.Error) error {
	return &diag.Error{
		Msg:   "Writing assets failed: " + reasonOf(failed) + ". Every change was undone.",
		File:  failed.File,
		Cause: failed.Cause,
		Hint:  failed.Hint,
	}
}

func errNotRestored(failure error, unrestored []string) error {
	return &diag.Error{
		Msg: "Writing assets failed (" + reasonOf(failure) + "), and these files could not be restored: " +
			strings.Join(unrestored, ", "),
		Cause: failure,
		Hint:  "Restore the map folder from version control before retrying.",
	}
}
