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
	write, err := planStateWrite(root, stateFile, result.State)
	if err != nil {
		return err
	}
	var journal fsx.Journal
	applyErr := applyAll(ctx, folder.WithChanges(result.Changes), write, &journal)
	if applyErr == nil {
		return nil
	}
	writeCount := journal.Len()
	return wrapUndone(applyErr, writeCount, describeUndoFailures(folder, journal.Undo()))
}

type stateWrite struct {
	stateFile    string
	fullPath     string
	previousData []byte
	existed      bool
	data         []byte
	remove       bool
}

func planStateWrite(root, stateFile string, state State) (*stateWrite, error) {
	fullPath, err := fsx.SafeJoinNoSymlinks(root, stateFile)
	if err != nil {
		return nil, err
	}
	existing, found, err := readStateFile(fullPath, stateFile)
	if err != nil {
		return nil, err
	}
	write := &stateWrite{stateFile: stateFile, fullPath: fullPath, previousData: existing, existed: found}
	if len(state.Files) == 0 {
		if !found {
			return nil, nil
		}
		write.remove = true
		return write, nil
	}
	write.data = state.Encode()
	if found && bytes.Equal(existing, write.data) {
		return nil, nil
	}
	return write, nil
}

func applyAll(ctx context.Context, view *mapdir.Folder, write *stateWrite, journal *fsx.Journal) error {
	if err := view.ApplyInPlace(ctx, journal); err != nil || write == nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return write.apply(journal)
}

func (w *stateWrite) apply(journal *fsx.Journal) error {
	err := w.checkUnchanged()
	if err != nil {
		return err
	}
	if w.remove {
		err = journal.Remove(w.fullPath)
	} else {
		err = journal.Write(w.fullPath, w.data)
	}
	if err != nil {
		return errStateNotWritten(w.stateFile, err)
	}
	return nil
}

func (w *stateWrite) checkUnchanged() error {
	current, found, err := readStateFile(w.fullPath, w.stateFile)
	switch {
	case err != nil:
		return err
	case found != w.existed || !bytes.Equal(current, w.previousData):
		return errStateChanged(w.stateFile)
	}
	return nil
}

func describeUndoFailures(folder *mapdir.Folder, failures []fsx.UndoFailure) []string {
	var descriptions []string
	for _, failure := range failures {
		descriptions = append(descriptions, mapDisplayPath(folder, failure.Path)+" ("+fsx.Reason(failure.Err)+")")
	}
	return descriptions
}

func mapDisplayPath(folder *mapdir.Folder, path string) string {
	dir, err := filepath.Abs(folder.Dir())
	if err != nil {
		return path
	}
	rel, err := filepath.Rel(dir, path)
	if err != nil || !filepath.IsLocal(rel) {
		return path
	}
	return folder.DisplayPath(filepath.ToSlash(rel))
}

func wrapUndone(err error, writeCount int, unrestored []string) error {
	var diagErr *diag.Error
	isExpected := errors.As(err, &diagErr)
	switch {
	case len(unrestored) > 0:
		return errNotRestored(err, unrestored)
	case isCancelled(err) && writeCount == 0:
		return errInterruptedBeforeWriting()
	case isCancelled(err):
		return errInterruptedAndUndone()
	case isExpected && diagErr.Cause != nil:
		return errNotWritten(diagErr)
	}
	return err
}

func isCancelled(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

func describeFailure(err error) string {
	var diagErr *diag.Error
	switch {
	case isCancelled(err):
		return "interrupted"
	case !errors.As(err, &diagErr):
		return fsx.Reason(err)
	case diagErr.Cause != nil:
		return fsx.Reason(diagErr.Cause)
	}
	return diagErr.Msg
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

func errStateChanged(stateFile string) error {
	return &diag.Error{
		Msg:  stateFile + " changed after the assets were checked.",
		File: stateFile,
		Hint: "Close World Editor and anything else writing to the map, then retry.",
	}
}

func errStateNotWritten(stateFile string, cause error) error {
	return &diag.Error{
		Msg:   "Writing the asset ownership state failed: " + fsx.Reason(cause),
		File:  stateFile,
		Cause: cause,
		Hint:  "Make sure the file and its folder can be written and no other program has the file open, then retry.",
	}
}

func errNotWritten(diagErr *diag.Error) error {
	return &diag.Error{
		Msg:   "Writing assets failed: " + describeFailure(diagErr) + ". Every change was undone.",
		File:  diagErr.File,
		Cause: diagErr.Cause,
		Hint:  diagErr.Hint,
	}
}

func errNotRestored(cause error, unrestored []string) error {
	return &diag.Error{
		Msg: "Writing assets failed (" + describeFailure(cause) + "), and these files could not be restored: " +
			strings.Join(unrestored, ", "),
		Cause: cause,
		Hint:  "Restore the map folder from version control before retrying.",
	}
}
