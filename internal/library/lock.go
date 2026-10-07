package library

import (
	"encoding/json"
	"maps"
	"slices"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/fsx"
)

// lockFile is the committed lock file, at the project root.
const lockFile = "moonwell.lock"

// lockEntry is what a GitHub library resolved to: the manifest's github, tag and dir, the tag's commit, and the
// hash of the module files kept from it.
type lockEntry struct {
	GitHub, Tag, Dir, Commit, Files string
	Assets                          *string // the hash of the files it ships for the map; nil when it ships none
}

// lockAt is the path on disk of the lock of the project at root. A link in its place is refused, and nothing is
// read or written: the lock is committed, so a project can come with a link there, and what is read through it
// is another file, and what is written through it lies outside the project.
func lockAt(root string) (string, error) { return fsx.Inside(root, lockFile) }

// readLock returns the lock's entries by library key; none when there is no lock file. A link in the lock's place
// is refused.
func readLock(root string) (map[string]lockEntry, error) {
	path, err := lockAt(root)
	if err != nil {
		return nil, err
	}
	data, found, err := fsx.ReadIfThere(path)
	switch {
	case err != nil:
		return nil, errUnreadableLock(err)
	case !found:
		return map[string]lockEntry{}, nil
	case !json.Valid(data):
		return nil, errLockNotJSON()
	}
	return entriesOf(data)
}

// entriesOf is the entries of a lock document, which is valid JSON: an object whose member libraries is an object
// of entries. Members that a lock does not have by its layout are passed over.
func entriesOf(document []byte) (map[string]lockEntry, error) {
	members, _ := objectOf(document)
	libraries, isObject := objectOf(members["libraries"])
	if !isObject {
		return nil, errNotALock()
	}
	entries := map[string]lockEntry{}
	for key, written := range libraries {
		entry, isEntry := entryOf(written)
		if !isEntry {
			return nil, errNotALock()
		}
		entries[key] = entry
	}
	return entries, nil
}

// entryOf reads a lock entry from the JSON object it is written as. It is false for a value that is no object,
// for an entry without one of github, tag, dir, commit and files as a string, and for one whose assets is there
// and is no string.
func entryOf(written json.RawMessage) (lockEntry, bool) {
	members, isObject := objectOf(written)
	if !isObject {
		return lockEntry{}, false
	}
	var entry lockEntry
	fields := map[string]*string{
		"github": &entry.GitHub, "tag": &entry.Tag, "dir": &entry.Dir, "commit": &entry.Commit, "files": &entry.Files,
	}
	for name, field := range fields {
		text, isString := stringOf(members[name])
		if !isString {
			return lockEntry{}, false
		}
		*field = text
	}
	if hash, given := members["assets"]; given {
		text, isString := stringOf(hash)
		if !isString {
			return lockEntry{}, false
		}
		entry.Assets = &text
	}
	return entry, true
}

// writeLock writes the entries sorted by key, and only when the file's text changes. No entries removes the file.
// A link in the lock's place is refused, and stays.
func writeLock(root string, entries map[string]lockEntry) error {
	path, err := lockAt(root)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		if err := fsx.RemoveFile(path); err != nil {
			return errUnwritableLock("Removing", err)
		}
		return nil
	}
	if _, err := fsx.WriteIfChanged(path, lockText(entries)); err != nil {
		return errUnwritableLock("Writing", err)
	}
	return nil
}

// lockText is the lock file of the entries: an object whose member libraries holds each entry under its key, the
// keys in byte order, with two spaces for each level and a line break at the end.
//
// The file is committed, so its text for the same libraries must stay the same byte for byte. Every string is
// therefore written by fsx.Quoted, which escapes only what JSON cannot hold as it is; encoding/json also writes
// the line and paragraph separators (U+2028, U+2029) as escapes, and a tag or a folder may hold one.
func lockText(entries map[string]lockEntry) string {
	var out strings.Builder
	out.WriteString("{\n  \"libraries\": {")
	for i, key := range slices.Sorted(maps.Keys(entries)) {
		if i > 0 {
			out.WriteByte(',')
		}
		out.WriteString("\n    " + fsx.Quoted(key) + ": " + objectText(entryMembers(entries[key]), "    "))
	}
	out.WriteString("\n  }\n}\n")
	return out.String()
}

// member is one member of a JSON object as it is written: its name, and its value as JSON.
type member struct{ name, value string }

// entryMembers is an entry as the members of the object it is written as: github, tag, dir, commit, files and,
// for a library that ships files for the map, assets.
func entryMembers(entry lockEntry) []member {
	members := []member{
		{"github", fsx.Quoted(entry.GitHub)},
		{"tag", fsx.Quoted(entry.Tag)},
		{"dir", fsx.Quoted(entry.Dir)},
		{"commit", fsx.Quoted(entry.Commit)},
		{"files", fsx.Quoted(entry.Files)},
	}
	if entry.Assets != nil {
		members = append(members, member{"assets", fsx.Quoted(*entry.Assets)})
	}
	return members
}

// objectText writes members as a JSON object, each on a line of its own, two spaces further in than indent, where
// the closing brace stands. It needs at least one member: without any, the braces stand around an empty line.
func objectText(members []member, indent string) string {
	lines := make([]string, len(members))
	for i, m := range members {
		lines[i] = indent + "  " + fsx.Quoted(m.name) + ": " + m.value
	}
	return "{\n" + strings.Join(lines, ",\n") + "\n" + indent + "}"
}

// ---- errors ----

const lockHint = "Fix it, or delete it: the next check downloads every library again and writes a new one."

func errUnreadableLock(cause error) error {
	return &diag.Error{Msg: "Reading " + lockFile + " failed: " + fsx.Reason(cause), File: lockFile, Hint: lockHint, Cause: cause}
}

func errLockNotJSON() error {
	return &diag.Error{Msg: lockFile + " is not valid JSON.", File: lockFile, Hint: lockHint}
}

func errNotALock() error {
	return &diag.Error{Msg: lockFile + " is not a Moonwell lock file.", File: lockFile, Hint: lockHint}
}

// errUnwritableLock is a failure to write the lock or to remove it; doing says which, as "Writing".
func errUnwritableLock(doing string, cause error) error {
	return &diag.Error{
		Msg:   doing + " " + lockFile + " failed: " + reasonOf(cause),
		File:  lockFile,
		Hint:  "Close programs that have " + lockFile + " open, and check it is not read-only.",
		Cause: cause,
	}
}
