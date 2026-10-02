package library

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/ordered"
	"github.com/mdlsvensson/moonwell/internal/text"
)

// LockFile is the committed lock file, at the project root.
const LockFile = "moonwell.lock"

// LockEntry is what a GitHub library resolved to: the manifest's github, tag and dir, the tag's commit, the hash
// of the kept module files and, for a library that ships assets, the hash of those.
type LockEntry struct {
	GitHub string
	Tag    string
	Dir    string
	Commit string
	Files  string
	Assets *string
}

const lockHint = "Fix it, or delete it: the next check downloads every library again and writes a new one."

var lockFields = []string{"github", "tag", "dir", "commit", "files"}

// entryOf reads a lock entry from a JSON object; false when a field is missing or of another type.
func entryOf(value any) (LockEntry, bool) {
	object, ok := value.(*ordered.Object)
	if !ok {
		return LockEntry{}, false
	}
	fields := make([]string, len(lockFields))
	for i, name := range lockFields {
		field, _ := object.Get(name)
		if fields[i], ok = field.(string); !ok {
			return LockEntry{}, false
		}
	}
	entry := LockEntry{GitHub: fields[0], Tag: fields[1], Dir: fields[2], Commit: fields[3], Files: fields[4]}
	if assets, given := object.Get("assets"); given {
		hash, isString := assets.(string)
		if !isString {
			return LockEntry{}, false
		}
		entry.Assets = &hash
	}
	return entry, true
}

// ReadLock returns the lock's entries by library key; none when there is no lock file.
func ReadLock(root string) (map[string]LockEntry, error) {
	entries := map[string]LockEntry{}
	data, err := os.ReadFile(filepath.Join(root, LockFile))
	if errors.Is(err, fs.ErrNotExist) {
		return entries, nil
	}
	if err != nil {
		return nil, &diag.Error{Msg: "Reading " + LockFile + " failed: " + fsx.Reason(err), File: LockFile, Cause: err, Hint: lockHint}
	}
	tree, err := ordered.Decode([]byte(text.Lossy(data)))
	if err != nil {
		return nil, &diag.Error{Msg: LockFile + " is not valid JSON.", File: LockFile, Cause: err, Hint: lockHint}
	}
	notALock := &diag.Error{Msg: LockFile + " is not a Moonwell lock file.", File: LockFile, Hint: lockHint}
	document, _ := tree.(*ordered.Object)
	if document == nil {
		return nil, notALock
	}
	listed, _ := document.Get("libraries")
	libraries, ok := listed.(*ordered.Object)
	if !ok {
		return nil, notALock
	}
	for key, value := range libraries.All() {
		entry, ok := entryOf(value)
		if !ok {
			return nil, notALock
		}
		entries[key] = entry
	}
	return entries, nil
}

// stampFields is a lock entry as the JSON object it is written as: github, tag, dir, commit, files and, when it has
// one, assets.
func stampFields(entry LockEntry) *ordered.Map[any] {
	fields := &ordered.Map[any]{}
	fields.Set("github", entry.GitHub)
	fields.Set("tag", entry.Tag)
	fields.Set("dir", entry.Dir)
	fields.Set("commit", entry.Commit)
	fields.Set("files", entry.Files)
	if entry.Assets != nil {
		fields.Set("assets", *entry.Assets)
	}
	return fields
}

// WriteLock writes the entries sorted by key, with two-space indentation, only when the text changes. No entries
// removes the file.
func WriteLock(root string, libraries map[string]LockEntry) error {
	path := filepath.Join(root, LockFile)
	keys := make([]string, 0, len(libraries))
	for key := range libraries {
		keys = append(keys, key)
	}
	if len(keys) == 0 {
		return fsx.RemoveFile(path)
	}
	text.Sort(keys)
	sorted := &ordered.Map[any]{}
	for _, key := range keys {
		sorted.Set(key, stampFields(libraries[key]))
	}
	document := &ordered.Map[any]{}
	document.Set("libraries", sorted)
	_, err := fsx.WriteIfChanged(path, ordered.Stringify(document, 2)+"\n")
	return err
}
