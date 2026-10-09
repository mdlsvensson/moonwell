package fsx

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
)

type Journal struct {
	entries []journalEntry
}

type journalEntry struct {
	path         string
	originalData []byte
	existed      bool
}

func (j *Journal) Write(path string, data []byte) error {
	if err := j.record(path); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o777); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o666)
}

func (j *Journal) Remove(path string) error {
	if err := j.record(path); err != nil {
		return err
	}
	return RemoveFile(path)
}

func (j *Journal) Len() int { return len(j.entries) }

type UndoFailure struct {
	Path string
	Err  error
}

func (j *Journal) Undo() (failures []UndoFailure) {
	for _, entry := range slices.Backward(j.entries) {
		if err := entry.restore(); err != nil {
			failures = append(failures, UndoFailure{Path: entry.path, Err: err})
		}
	}
	j.entries = nil
	return failures
}

func (j *Journal) record(path string) error {
	originalData, existed, err := ReadFileIfExists(path)
	if err != nil {
		return err
	}
	j.entries = append(j.entries, journalEntry{path: path, originalData: originalData, existed: existed})
	return nil
}

func (e journalEntry) restore() error {
	if e.isUnchanged() {
		return nil
	}
	if !e.existed {
		return RemoveFile(e.path)
	}
	return os.WriteFile(e.path, e.originalData, 0o666)
}

func (e journalEntry) isUnchanged() bool {
	currentData, found, err := ReadFileIfExists(e.path)
	return err == nil && found == e.existed && bytes.Equal(currentData, e.originalData)
}
