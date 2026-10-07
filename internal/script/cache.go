package script

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/fsx"
)

// hashesFile is the file of the staging folder in which a compile keeps what it left there, for the next one.
const hashesFile = ".hashes.json"

// dependsOn is what every output of a compile depends on beside its own source. A compile with another value
// compiles every source again.
type dependsOn struct {
	Compiler     string `json:"compiler"`     // the compiler's path
	Mode         string `json:"mode"`         // -r or -m
	Macros       string `json:"macros"`       // the SHA-256 of the macro module
	MacroSources string `json:"macroSources"` // macroSourcesOf the YueScript sources; "" when none may define a macro
}

// macroSourcesOf is the member of what every output depends on that stands for the macros the project and its
// libraries write themselves: the SHA-256 over the path and the hash, in the order of the paths' bytes, of every
// YueScript source that holds the word `macro`; "" when no source holds it.
//
// A source imports macros from another source (`import "m" as {:$N}`), and what it compiles to, and the globals
// it uses, are then that other source's as much as its own. Which source imports which is the compiler's to
// know, so every source that may define a macro counts for every output: an edit, an arrival or a removal of
// one compiles every source again, and an edit of any other source compiles that source alone.
//
// It follows the sources that Collect found, and nothing else. A file that the body of a macro loads while the
// compiler runs, and a macro module that the compiler finds outside the project's and the libraries' sources,
// may change what a source compiles to without changing this.
func macroSourcesOf(units []unit) string {
	var holding []unit
	for _, u := range units {
		if holdsMacroWord(u.text) {
			holding = append(holding, u)
		}
	}
	if len(holding) == 0 {
		return ""
	}
	slices.SortFunc(holding, func(a, b unit) int { return strings.Compare(a.path, b.path) })
	var hashed strings.Builder
	for _, u := range holding {
		hashed.WriteString(u.path + "\x00" + u.hash + "\n")
	}
	return fsx.SHA256Hex([]byte(hashed.String()))
}

// holdsMacroWord reports whether a text holds the word `macro`: those five bytes with no letter, digit or "_"
// of ASCII directly before or after. A source that defines a macro for others holds it (`export macro N`), and
// so does one that only says the word in a comment or a string: the test errs on the side of yes. `macros`, as
// in `import "moonwell.macros"`, is another word.
func holdsMacroWord(text string) bool {
	const word = "macro"
	for from := 0; ; {
		at := strings.Index(text[from:], word)
		if at < 0 {
			return false
		}
		start, end := from+at, from+at+len(word)
		if (start == 0 || !joinsAWord(text[start-1])) && (end == len(text) || !joinsAWord(text[end])) {
			return true
		}
		from = start + 1
	}
}

// joinsAWord reports whether a byte beside a word makes it another word: a letter, a digit or "_" of ASCII.
func joinsAWord(b byte) bool {
	return b == '_' || (b >= '0' && b <= '9') || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

// hashes is the content of the hashes file: what the outputs depend on, and each source whose output is good,
// by its path from the project folder.
type hashes struct {
	dependsOn
	Sources map[string]keptSource `json:"sources"`
}

// keptSource is a source as the hashes file keeps it.
type keptSource struct {
	Hash   string `json:"hash"`   // the SHA-256 of the source's bytes; "" while nothing vouches for its output
	Output string `json:"output"` // where its Lua is, from the staging folder, with "/"
}

// readHashes is what the last compile kept. Without a file, or with one in another shape, nothing is kept: every
// source is compiled again and no output is removed. A link on the way to the file is refused.
func readHashes(root string) (hashes, error) {
	kept, found, err := readCache[hashes](root, hashesFile)
	if err != nil || !found || !kept.namesOutputs() {
		return hashes{}, err
	}
	return kept, nil
}

// namesOutputs reports whether every output the file names is a Lua file below the staging folder, by the
// system's own rule for a path that stays below its folder. A compile joins the outputs it finds there with the
// folder and removes them, so a file that names anything else is none of its own.
func (h hashes) namesOutputs() bool {
	for _, source := range h.Sources {
		if !filepath.IsLocal(filepath.FromSlash(source.Output)) || !strings.HasSuffix(source.Output, ".lua") {
			return false
		}
	}
	return true
}

// stale parts the units into those that are to be compiled and those that are up to date. Every unit is stale
// when what the outputs depend on changed, and else the ones whose source changed or whose output is not there.
// A source that is kept without a hash is stale whatever its text.
func (h hashes) stale(units []unit, now dependsOn) (stale, upToDate []unit) {
	for _, u := range units {
		if h.dependsOn == now && h.Sources[u.path] == (keptSource{Hash: u.hash, Output: u.under}) && fsx.Exists(u.output) {
			upToDate = append(upToDate, u)
		} else {
			stale = append(stale, u)
		}
	}
	return stale, upToDate
}

// removeGone removes each output of the last compile that no source compiles to now: that of a source that is
// gone, and that of a library's module whose library has another key. stage is the staging folder on disk.
func removeGone(stage string, last hashes, units []unit) error {
	current := map[string]bool{}
	for _, u := range units {
		current[u.under] = true
	}
	for _, path := range slices.Sorted(maps.Keys(last.Sources)) {
		under := last.Sources[path].Output
		if current[under] {
			continue
		}
		if err := removeOutput(under, filepath.Join(stage, filepath.FromSlash(under))); err != nil {
			return err
		}
	}
	return nil
}

// writeHashes keeps the units whose Lua is at their outputs, with what those outputs depend on, and the units
// that are about to be compiled, each with its output and without a hash. The file vouches for no source without
// a hash, and still says where its output is: so a run that is stopped leaves nothing up to date that it was to
// compile, and the output of such a source is removed once the source is gone.
func writeHashes(root string, now dependsOn, good, pending []unit) error {
	kept := hashes{dependsOn: now, Sources: map[string]keptSource{}}
	for _, u := range good {
		kept.Sources[u.path] = keptSource{Hash: u.hash, Output: u.under}
	}
	for _, u := range pending {
		kept.Sources[u.path] = keptSource{Output: u.under}
	}
	return writeCache(root, hashesFile, kept)
}

// usesFile is the file of the staging folder in which a check keeps the globals each source uses, for the next
// one.
const usesFile = ".globals.json"

// listedWith is what every list of uses depends on beside its own source. A check with another value lists the
// uses of every source again.
type listedWith struct {
	Compiler     string `json:"compiler"`     // the compiler's path
	Macros       string `json:"macros"`       // the SHA-256 of the macro module
	MacroSources string `json:"macroSources"` // macroSourcesOf the YueScript sources, as the compile found them
}

// keptUses is the content of the uses file: what the lists depend on, and each source that was listed, by its
// path from the project folder.
type keptUses struct {
	listedWith
	Sources map[string]sourceUses `json:"sources"`
}

// sourceUses is a source as the uses file keeps it.
type sourceUses struct {
	Hash string `json:"hash"` // the SHA-256 of the source's bytes
	// Uses is the globals it uses, in the order the compiler lists them; never nil. A name goes through JSON, so
	// one with bytes that are not UTF-8 would come back changed; no valid source gives such a name.
	Uses []globalUse `json:"uses"`
}

// readUses is the lists of the last check that hold now: each source's, by its path, when they were made with
// what the lists depend on now. Without a file, with one in another shape or with one made with something else,
// there are none, and every source is listed again. A link on the way to the file is refused.
func readUses(root string, now listedWith) (map[string]sourceUses, error) {
	kept, found, err := readCache[keptUses](root, usesFile)
	if err != nil || !found || kept.listedWith != now || !kept.listsEverySource() {
		return map[string]sourceUses{}, err
	}
	return kept.Sources, nil
}

// listsEverySource reports whether the file has a list of uses for each source it keeps, and a name for each
// use. A source without a list would pass for one that uses no global, and its unknown globals for none: a file
// that has such a source is none of this shape.
func (k keptUses) listsEverySource() bool {
	for _, source := range k.Sources {
		if source.Uses == nil || slices.ContainsFunc(source.Uses, func(use globalUse) bool { return use.Name == "" }) {
			return false
		}
	}
	return k.Sources != nil
}

// writeUses keeps the lists of uses, each with the hash of the source it was made of, and what they depend on.
//
// A list is data and no file on disk: it is kept once it is made, and never while it is being made. So a check
// that is stopped leaves the file as it was, which is true of every source it names, and the file needs no entry
// for a source that is about to be listed, as the hashes file has for one that is about to be compiled.
func writeUses(root string, now listedWith, lists map[string]sourceUses) error {
	return writeCache(root, usesFile, keptUses{listedWith: now, Sources: lists})
}

// readCache reads a file that a compile keeps in the staging folder, as the T it was written from. found is
// false without such a file: where there is none, where it cannot be read, and where it is no JSON in the shape
// of T, which is so for text that is no JSON value, for a member T has not, for a value of another kind than
// T's, and for anything after the value. A link on the way to the file is refused.
func readCache[T any](root, name string) (kept T, found bool, err error) {
	var none T
	file, err := placeOf(root, stageDir+"/"+name, errUnreadableOutput)
	if err != nil {
		return none, false, err
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return none, false, nil
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&kept) != nil {
		return none, false, nil
	}
	if _, err := decoder.Token(); err != io.EOF {
		return none, false, nil
	}
	return kept, true, nil
}

// writeCache writes a file that a compile keeps in the staging folder: kept as JSON, with two spaces for each
// level and a line break at the end. A link on the way to the file is refused.
func writeCache(root, name string, kept any) error {
	path := stageDir + "/" + name
	text, err := json.MarshalIndent(kept, "", "  ")
	if err != nil {
		// A plain error: what is kept is a struct of strings, numbers and maps of them, which is always JSON, so
		// a value that is not is a mistake in Moonwell.
		return fmt.Errorf("script: %s cannot be written as JSON: %w", path, err)
	}
	file, err := placeOf(root, path, errUnwritableOutput)
	if err != nil {
		return err
	}
	if _, err := fsx.WriteIfChanged(file, string(text)+"\n"); err != nil {
		return errUnwritableOutput(path, err)
	}
	return nil
}

// ---- errors ----

// The failures of this file are those of a file below the staging folder that cannot be read, written or
// removed. They are worded where the compile words them for its outputs: errUnreadableOutput,
// errUnwritableOutput and errUnremovableOutput, below the same line of yue.go.
