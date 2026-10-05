package script

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"os"
	"slices"
	"strings"

	"github.com/mdlsvensson/moonwell/next/internal/fsx"
)

// hashesFile is the file of the staging folder in which a compile keeps what it left there, for the next one.
const hashesFile = ".hashes.json"

// dependsOn is what every output of a compile depends on beside its own source. A compile with another value
// compiles every source again.
type dependsOn struct {
	Compiler string `json:"compiler"` // the compiler's path
	Mode     string `json:"mode"`     // -r or -m
	Macros   string `json:"macros"`   // the SHA-256 of the macro module
}

// hashes is the content of the hashes file: what the outputs depend on, and each source whose output is good,
// by its path from the project folder.
type hashes struct {
	dependsOn
	Sources map[string]keptSource `json:"sources"`
}

// keptSource is a source as the hashes file keeps it.
type keptSource struct {
	Hash   string `json:"hash"`   // the SHA-256 of the source's bytes
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

// namesOutputs reports whether every output the file names is a Lua file below the staging folder, written as a
// compile writes it. A compile removes the outputs it finds there, so a file that names anything else is none
// of its own.
func (h hashes) namesOutputs() bool {
	for _, source := range h.Sources {
		path, ok := fsx.RelPath(source.Output)
		if !ok || path != source.Output || !strings.HasSuffix(path, ".lua") {
			return false
		}
	}
	return true
}

// stale is the units that are to be compiled: every unit when what the outputs depend on changed, and else the
// ones whose source changed or whose output is not there.
func (h hashes) stale(units []unit, now dependsOn) []unit {
	var stale []unit
	for _, u := range units {
		upToDate := h.dependsOn == now && h.Sources[u.path] == keptSource{Hash: u.hash, Output: u.under} && fsx.Exists(u.output)
		if !upToDate {
			stale = append(stale, u)
		}
	}
	return stale
}

// removeGone removes each output of the last compile that no source compiles to now: that of a source that is
// gone, and that of a library's module whose library has another key.
func removeGone(root string, last hashes, units []unit) error {
	current := map[string]bool{}
	for _, u := range units {
		current[u.under] = true
	}
	for _, path := range slices.Sorted(maps.Keys(last.Sources)) {
		under := last.Sources[path].Output
		if current[under] {
			continue
		}
		output, err := placeOf(root, stageDir+"/"+under, errUnremovableOutput)
		if err != nil {
			return err
		}
		if err := removeOutput(under, output); err != nil {
			return err
		}
	}
	return nil
}

// writeHashes keeps the units whose Lua is at their outputs, with what those outputs depend on.
func writeHashes(root string, now dependsOn, units []unit) error {
	kept := hashes{dependsOn: now, Sources: map[string]keptSource{}}
	for _, u := range units {
		kept.Sources[u.path] = keptSource{Hash: u.hash, Output: u.under}
	}
	return writeCache(root, hashesFile, kept)
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
