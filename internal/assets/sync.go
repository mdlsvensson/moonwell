package assets

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/mapdir"
)

// Sync writes a plan's changes into the map folder itself and the ownership state into stateFile. When a write
// fails or ctx is cancelled it puts back every file it changed.
//
// folder is the folder the plan was made from. What Plan read through it is what each file is checked against
// before it is replaced or removed, so a file that changed after the plan stops the sync and is not written
// over; another folder has no such record, and is refused as the caller's bug.
//
// A sync writes the assets' changes and no other. A folder that is a view with planned changes, such as the
// settings' in a build, would have those written into the source map too: it is refused as the caller's bug,
// before anything is read or written. Plan takes such a view; a build stages it.
//
// The state file is written last, so that it never lists a file that was not written. A state that owns nothing
// has no file: one that is there is removed. A state file that is written or removed is guarded as a map file
// is: one that another program wrote, made or removed after the sync began stops the sync, and is kept. A state
// file that holds the state already is not written, and not looked at again. The folder of the state file is
// made when it is needed. Folders made for new files stay when the files are taken out again.
func Sync(ctx context.Context, folder *mapdir.Folder, result *Result, stateFile string) error {
	if result.planned != folder {
		return errOtherFolder()
	}
	if len(folder.Changes()) > 0 {
		return errFolderWithChanges()
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

// stateWrite is what a sync does to the state file: it writes bytes, or removes the file. It keeps what the file
// was when the sync began, to compare with what the file is just before the write or the removal.
type stateWrite struct {
	file   string
	held   []byte // what the file held when the sync began
	found  bool   // whether it was there
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
	change := &stateWrite{file: file, held: held, found: found}
	if len(state.Files) == 0 {
		if !found {
			return nil, nil
		}
		change.remove = true
		return change, nil
	}
	change.bytes = state.Bytes()
	if found && bytes.Equal(held, change.bytes) {
		return nil, nil
	}
	return change, nil
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

// apply writes or removes the state file through journal, unless the file is not what it was when the sync began.
func (s *stateWrite) apply(journal *fsx.Journal) error {
	err := s.asItWas()
	if err != nil {
		return err
	}
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

// asItWas fails unless the state file is what it was when the sync began: there with the same bytes, or not
// there. Another program that wrote, made or removed it in between has its file kept.
func (s *stateWrite) asItWas() error {
	held, found, err := readIfThere(s.file)
	switch {
	case err != nil:
		return err
	case found != s.found || !bytes.Equal(held, s.held):
		return errStateChanged(s.file)
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

// errFolderWithChanges is not a diag error: assets:sync plans on the source map as it is on disk, so only a
// caller's bug gives Sync a view that holds the changes of another planner.
func errFolderWithChanges() error {
	return errors.New("Cannot sync the assets: the folder carries planned changes, which a sync would write too.")
}

func errInterruptedAndUndone() error {
	return &diag.Error{Msg: "Interrupted; every change was undone."}
}

// errStateChanged is mapdir's refusal of a map file that changed after the plan, for the state file, which is
// named by its path.
func errStateChanged(file string) error {
	return &diag.Error{
		Msg:  file + " changed after the assets were checked.",
		File: file,
		Hint: "Close World Editor and anything else writing to the map, then retry.",
	}
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
