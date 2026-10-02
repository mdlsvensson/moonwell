package yue

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"sync"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/ordered"
	"github.com/mdlsvensson/moonwell/internal/proc"
	"github.com/mdlsvensson/moonwell/internal/text"
)

// GlobalUse is one global a source file reads or writes, at its 1-based position.
type GlobalUse struct {
	Name   string
	Line   int
	Column int
}

// UsesCache holds the `yue -g` results, next to the compile hashes.
const UsesCache = "dist/stage/lua/.globals.json"

var useLine = regexp.MustCompile(`^([^` + text.SpaceSet + `]+) ([0-9]+) ([0-9]+)$`)

// ParseGlobalUses reads `yue -g` output: one `NAME LINE COLUMN` per line. file names the source in errors.
func ParseGlobalUses(output, file string) ([]GlobalUse, error) {
	uses := []GlobalUse{}
	for _, raw := range lineBreak.Split(output, -1) {
		line := text.Trim(raw)
		if line == "" {
			continue
		}
		match := useLine.FindStringSubmatch(line)
		if match == nil {
			return nil, &diag.Error{
				Msg:  "yue -g printed a line Moonwell cannot read: " + line,
				File: file,
				Hint: "Use a YueScript version Moonwell supports: remove yue.version and yue.path from the manifests.",
			}
		}
		lineNumber, _ := strconv.Atoi(match[2])
		column, _ := strconv.Atoi(match[3])
		uses = append(uses, GlobalUse{Name: match[1], Line: lineNumber, Column: column})
	}
	return uses, nil
}

// usesEntry is one file of the cache: the hash its uses were listed for.
type usesEntry struct {
	hash string
	uses []GlobalUse
}

// readUses reads the cache; nothing when it is missing or is not what ListGlobalUses writes.
func readUses(path string) (settings string, files map[string]usesEntry) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", nil
	}
	tree, err := ordered.Decode(data)
	if err != nil {
		return "", nil
	}
	document, ok := tree.(*ordered.Object)
	if !ok {
		return "", nil
	}
	setting, _ := document.Get("settings")
	listed, _ := document.Get("files")
	settings, isString := setting.(string)
	entries, isObject := listed.(*ordered.Object)
	if !isString || !isObject {
		return "", nil
	}
	files = map[string]usesEntry{}
	for file, value := range entries.All() {
		entry, ok := usesEntryOf(value)
		if !ok {
			return "", nil
		}
		files[file] = entry
	}
	return settings, files
}

func usesEntryOf(value any) (usesEntry, bool) {
	fields, ok := value.(*ordered.Object)
	if !ok {
		return usesEntry{}, false
	}
	hash, _ := fields.Get("hash")
	listed, _ := fields.Get("uses")
	entry := usesEntry{uses: []GlobalUse{}}
	rows, isList := listed.([]any)
	if entry.hash, ok = hash.(string); !ok || !isList {
		return usesEntry{}, false
	}
	for _, row := range rows {
		cells, _ := row.([]any)
		if len(cells) != 3 {
			return usesEntry{}, false
		}
		name, isName := cells[0].(string)
		line, isLine := cells[1].(float64)
		column, isColumn := cells[2].(float64)
		if !isName || !isLine || !isColumn {
			return usesEntry{}, false
		}
		entry.uses = append(entry.uses, GlobalUse{Name: name, Line: int(line), Column: int(column)})
	}
	return entry, true
}

// UsesOptions say which sources to list the globals of.
type UsesOptions struct {
	// Yue is the path of the compiler.
	Yue  string
	Root string
	// Hashes are the sources, by POSIX path under src/, each with the hash of its text.
	Hashes map[string]string
	Macros *MacroSearch
	// Run runs the compiler; nil is proc.Run.
	Run proc.RunFunc
	// Concurrency is how many compilers run at once; 0 is eight.
	Concurrency int
}

// ListGlobalUses gives the globals each source uses, keyed like Hashes. It runs `yue -g` only for the files whose
// hash or compiler changed since the last run, and for every file when the macro module changed. `yue -g` cannot
// share a run with compiling, so it runs separately.
func ListGlobalUses(ctx context.Context, options UsesOptions) (map[string][]GlobalUse, error) {
	run := options.Run
	if run == nil {
		run = proc.Run
	}
	cachePath := filepath.Join(options.Root, filepath.FromSlash(UsesCache))
	previousSettings, previous := readUses(cachePath)
	settings := options.Yue
	if options.Macros != nil {
		settings += "|" + options.Macros.Hash
	}
	names := make([]string, 0, len(options.Hashes))
	for file := range options.Hashes {
		names = append(names, file)
	}
	text.Sort(names)
	files := map[string]usesEntry{}
	var pending []string
	for _, file := range names {
		if cached, found := previous[file]; found && previousSettings == settings && cached.hash == options.Hashes[file] {
			files[file] = cached
		} else {
			pending = append(pending, file)
		}
	}

	var lock sync.Mutex
	var failures []*diag.Error
	fail := func(failure *diag.Error) {
		lock.Lock()
		defer lock.Unlock()
		failures = append(failures, failure)
	}
	err := forEachLimited(pending, options.Concurrency, func(file string) error {
		label := "src/" + file
		args := append([]string{"-g"}, options.Macros.PathArgs()...)
		args = append(args, filepath.Join(options.Root, "src", filepath.FromSlash(file)))
		result, err := run(ctx, options.Yue, args, proc.Options{})
		if err != nil {
			return err
		}
		if result.Code != 0 {
			fail(CompileError(label, result.Stdout+"\n"+result.Stderr))
			return nil
		}
		uses, err := ParseGlobalUses(result.Stdout, label)
		if err != nil {
			// Collected like a failed run, so the other files finish and the ones that succeeded are cached.
			fail(err.(*diag.Error))
			return nil
		}
		lock.Lock()
		defer lock.Unlock()
		files[file] = usesEntry{hash: options.Hashes[file], uses: uses}
		return nil
	})
	if err != nil {
		return nil, err
	}

	listed := &ordered.Map[any]{}
	result := map[string][]GlobalUse{}
	for _, file := range names {
		entry, done := files[file]
		if !done {
			continue
		}
		rows := make([]any, len(entry.uses))
		for i, use := range entry.uses {
			rows[i] = []any{use.Name, use.Line, use.Column}
		}
		fields := &ordered.Map[any]{}
		fields.Set("hash", entry.hash)
		fields.Set("uses", rows)
		listed.Set(file, fields)
		result[file] = entry.uses
	}
	document := &ordered.Map[any]{}
	document.Set("settings", settings)
	document.Set("files", listed)
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o777); err != nil {
		return nil, err
	}
	if err := os.WriteFile(cachePath, []byte(ordered.Stringify(document, 2)), 0o666); err != nil {
		return nil, err
	}
	if len(failures) > 0 {
		return nil, firstFailure(failures)
	}
	return result, nil
}
