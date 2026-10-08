package library

import (
	"encoding/json"
	"maps"
	"slices"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/fsx"
)

const lockFile = "moonwell.lock"

type lockEntry struct {
	GitHub, Tag, Dir, Commit, Files string
	Assets                          *string
}

func lockPath(root string) (string, error) { return fsx.SafeJoinNoSymlinks(root, lockFile) }

func readLock(root string) (map[string]lockEntry, error) {
	path, err := lockPath(root)
	if err != nil {
		return nil, err
	}
	data, found, err := fsx.ReadFileIfExists(path)
	switch {
	case err != nil:
		return nil, errUnreadableLock(err)
	case !found:
		return map[string]lockEntry{}, nil
	case !json.Valid(data):
		return nil, errLockNotJSON()
	}
	return parseLock(data)
}

func parseLock(document []byte) (map[string]lockEntry, error) {
	members, _ := asObject(document)
	libraries, isObject := asObject(members["libraries"])
	if !isObject {
		return nil, errNotALock()
	}
	entries := map[string]lockEntry{}
	for key, written := range libraries {
		entry, isEntry := parseLockEntry(written)
		if !isEntry {
			return nil, errNotALock()
		}
		entries[key] = entry
	}
	return entries, nil
}

func parseLockEntry(written json.RawMessage) (lockEntry, bool) {
	members, isObject := asObject(written)
	if !isObject {
		return lockEntry{}, false
	}
	var entry lockEntry
	fields := map[string]*string{
		"github": &entry.GitHub, "tag": &entry.Tag, "dir": &entry.Dir, "commit": &entry.Commit, "files": &entry.Files,
	}
	for name, field := range fields {
		text, isString := asString(members[name])
		if !isString {
			return lockEntry{}, false
		}
		*field = text
	}
	if hash, given := members["assets"]; given {
		text, isString := asString(hash)
		if !isString {
			return lockEntry{}, false
		}
		entry.Assets = &text
	}
	return entry, true
}

func writeLock(root string, entries map[string]lockEntry) error {
	path, err := lockPath(root)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		if err := fsx.RemoveFile(path); err != nil {
			return errUnwritableLock("Removing", err)
		}
		return nil
	}
	if _, err := fsx.WriteIfChanged(path, formatLock(entries)); err != nil {
		return errUnwritableLock("Writing", err)
	}
	return nil
}

func formatLock(entries map[string]lockEntry) string {
	var out strings.Builder
	out.WriteString("{\n  \"libraries\": {")
	for i, key := range slices.Sorted(maps.Keys(entries)) {
		if i > 0 {
			out.WriteByte(',')
		}
		out.WriteString("\n    " + fsx.QuoteJSON(key) + ": " + formatObject(lockEntryMembers(entries[key]), "    "))
	}
	out.WriteString("\n  }\n}\n")
	return out.String()
}

type member struct{ name, value string }

func lockEntryMembers(entry lockEntry) []member {
	members := []member{
		{"github", fsx.QuoteJSON(entry.GitHub)},
		{"tag", fsx.QuoteJSON(entry.Tag)},
		{"dir", fsx.QuoteJSON(entry.Dir)},
		{"commit", fsx.QuoteJSON(entry.Commit)},
		{"files", fsx.QuoteJSON(entry.Files)},
	}
	if entry.Assets != nil {
		members = append(members, member{"assets", fsx.QuoteJSON(*entry.Assets)})
	}
	return members
}

func formatObject(members []member, indent string) string {
	lines := make([]string, len(members))
	for i, m := range members {
		lines[i] = indent + "  " + fsx.QuoteJSON(m.name) + ": " + m.value
	}
	return "{\n" + strings.Join(lines, ",\n") + "\n" + indent + "}"
}

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

func errUnwritableLock(doing string, cause error) error {
	return &diag.Error{
		Msg:   doing + " " + lockFile + " failed: " + describeFetchError(cause),
		File:  lockFile,
		Hint:  "Close programs that have " + lockFile + " open, and check it is not read-only.",
		Cause: cause,
	}
}
