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

func Sync(ctx context.Context, folder *mapdir.Folder, result *Result, root, stateFile string) error {
	if result.view != folder {
		return errOtherFolder()
	}
	if len(folder.Changes()) > 0 {
		return errFolderWithChanges()
	}
	state, err := planStateWrite(root, stateFile, result.State)
	if err != nil {
		return err
	}
	var journal fsx.Journal
	failure := applyAll(ctx, folder.WithChanges(result.Changes), state, &journal)
	if failure == nil {
		return nil
	}
	touched := journal.Len()
	return wrapUndone(failure, touched, describeUndoFailures(folder, journal.Undo()))
}

type stateWrite struct {
	file     string
	fullPath string
	previous []byte
	found    bool
	data     []byte
	remove   bool
}

func planStateWrite(root, file string, state State) (*stateWrite, error) {
	place, err := fsx.SafeJoinNoSymlinks(root, file)
	if err != nil {
		return nil, err
	}
	held, found, err := readStateFile(place, file)
	if err != nil {
		return nil, err
	}
	change := &stateWrite{file: file, fullPath: place, previous: held, found: found}
	if len(state.Files) == 0 {
		if !found {
			return nil, nil
		}
		change.remove = true
		return change, nil
	}
	change.data = state.Encode()
	if found && bytes.Equal(held, change.data) {
		return nil, nil
	}
	return change, nil
}

func applyAll(ctx context.Context, view *mapdir.Folder, state *stateWrite, journal *fsx.Journal) error {
	if err := view.ApplyInPlace(ctx, journal); err != nil || state == nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return state.apply(journal)
}

func (s *stateWrite) apply(journal *fsx.Journal) error {
	err := s.checkUnchanged()
	if err != nil {
		return err
	}
	if s.remove {
		err = journal.Remove(s.fullPath)
	} else {
		err = journal.Write(s.fullPath, s.data)
	}
	if err != nil {
		return errStateNotWritten(s.file, err)
	}
	return nil
}

func (s *stateWrite) checkUnchanged() error {
	held, found, err := readStateFile(s.fullPath, s.file)
	switch {
	case err != nil:
		return err
	case found != s.found || !bytes.Equal(held, s.previous):
		return errStateChanged(s.file)
	}
	return nil
}

func describeUndoFailures(folder *mapdir.Folder, files []fsx.UndoFailure) []string {
	var listed []string
	for _, file := range files {
		listed = append(listed, displayName(folder, file.Path)+" ("+fsx.Reason(file.Err)+")")
	}
	return listed
}

func displayName(folder *mapdir.Folder, path string) string {
	dir, err := filepath.Abs(folder.Dir())
	if err != nil {
		return path
	}
	below, err := filepath.Rel(dir, path)
	if err != nil || !filepath.IsLocal(below) {
		return path
	}
	return folder.DisplayPath(filepath.ToSlash(below))
}

func wrapUndone(failure error, touched int, unrestored []string) error {
	var expected *diag.Error
	isExpected := errors.As(failure, &expected)
	switch {
	case len(unrestored) > 0:
		return errNotRestored(failure, unrestored)
	case isCancelled(failure) && touched == 0:
		return errInterruptedBeforeWriting()
	case isCancelled(failure):
		return errInterruptedAndUndone()
	case isExpected && expected.Cause != nil:
		return errNotWritten(expected)
	}
	return failure
}

func isCancelled(failure error) bool {
	return errors.Is(failure, context.Canceled) || errors.Is(failure, context.DeadlineExceeded)
}

func describeFailure(failure error) string {
	var expected *diag.Error
	switch {
	case isCancelled(failure):
		return "interrupted"
	case !errors.As(failure, &expected):
		return fsx.Reason(failure)
	case expected.Cause != nil:
		return fsx.Reason(expected.Cause)
	}
	return expected.Msg
}

func errOtherFolder() error {
	return errors.New("Cannot sync the assets: the folder is not the folder the plan was made from.")
}

func errFolderWithChanges() error {
	return errors.New("Cannot sync the assets: the folder carries planned changes, which a sync would write too.")
}

func errInterruptedAndUndone() error {
	return &diag.Error{Msg: "Interrupted; every change was undone."}
}

func errStateChanged(file string) error {
	return &diag.Error{
		Msg:  file + " changed after the assets were checked.",
		File: file,
		Hint: "Close World Editor and anything else writing to the map, then retry.",
	}
}

func errStateNotWritten(file string, cause error) error {
	return &diag.Error{
		Msg:   "Writing the asset ownership state failed: " + fsx.Reason(cause),
		File:  file,
		Cause: cause,
		Hint:  "Make sure the file and its folder can be written and no other program has the file open, then retry.",
	}
}

func errNotWritten(failed *diag.Error) error {
	return &diag.Error{
		Msg:   "Writing assets failed: " + describeFailure(failed) + ". Every change was undone.",
		File:  failed.File,
		Cause: failed.Cause,
		Hint:  failed.Hint,
	}
}

func errNotRestored(failure error, unrestored []string) error {
	return &diag.Error{
		Msg: "Writing assets failed (" + describeFailure(failure) + "), and these files could not be restored: " +
			strings.Join(unrestored, ", "),
		Cause: failure,
		Hint:  "Restore the map folder from version control before retrying.",
	}
}
