package fsx

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
)

type Journal struct {
	touched []touch
}

type touch struct {
	path    string
	before  []byte
	existed bool
}

func (j *Journal) Write(path string, data []byte) error {
	if err := j.remember(path); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o777); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o666)
}

func (j *Journal) Remove(path string) error {
	if err := j.remember(path); err != nil {
		return err
	}
	return RemoveFile(path)
}

func (j *Journal) Len() int { return len(j.touched) }

type Unrestored struct {
	Path string
	Err  error
}

func (j *Journal) Undo() (unrestored []Unrestored) {
	for _, t := range slices.Backward(j.touched) {
		if err := t.restore(); err != nil {
			unrestored = append(unrestored, Unrestored{Path: t.path, Err: err})
		}
	}
	j.touched = nil
	return unrestored
}

func (j *Journal) remember(path string) error {
	before, existed, err := ReadIfThere(path)
	if err != nil {
		return err
	}
	j.touched = append(j.touched, touch{path: path, before: before, existed: existed})
	return nil
}

func (t touch) restore() error {
	if t.asItWas() {
		return nil
	}
	if !t.existed {
		return RemoveFile(t.path)
	}
	return os.WriteFile(t.path, t.before, 0o666)
}

func (t touch) asItWas() bool {
	now, found, err := ReadIfThere(t.path)
	return err == nil && found == t.existed && bytes.Equal(now, t.before)
}
