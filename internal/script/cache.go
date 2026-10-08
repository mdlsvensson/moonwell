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

const hashesFile = ".hashes.json"

type dependsOn struct {
	Compiler     string `json:"compiler"`
	Mode         string `json:"mode"`
	Macros       string `json:"macros"`
	MacroSources string `json:"macroSources"`
}

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

func joinsAWord(b byte) bool {
	return b == '_' || (b >= '0' && b <= '9') || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

type hashes struct {
	dependsOn
	Sources map[string]keptSource `json:"sources"`
}

type keptSource struct {
	Hash   string `json:"hash"`
	Output string `json:"output"`
}

func readHashes(root string) (hashes, error) {
	kept, found, err := readCache[hashes](root, hashesFile)
	if err != nil || !found || !kept.namesOutputs() {
		return hashes{}, err
	}
	return kept, nil
}

func (h hashes) namesOutputs() bool {
	for _, source := range h.Sources {
		if !filepath.IsLocal(filepath.FromSlash(source.Output)) || !strings.HasSuffix(source.Output, ".lua") {
			return false
		}
	}
	return true
}

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

func removeGone(outputs string, last hashes, units []unit) error {
	current := map[string]bool{}
	for _, u := range units {
		current[u.under] = true
	}
	for _, path := range slices.Sorted(maps.Keys(last.Sources)) {
		under := last.Sources[path].Output
		if current[under] {
			continue
		}
		if err := removeOutput(under, filepath.Join(outputs, filepath.FromSlash(under))); err != nil {
			return err
		}
	}
	return nil
}

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

const usesFile = ".globals.json"

type listedWith struct {
	Compiler     string `json:"compiler"`
	Macros       string `json:"macros"`
	MacroSources string `json:"macroSources"`
}

type keptUses struct {
	listedWith
	Sources map[string]sourceUses `json:"sources"`
}

type sourceUses struct {
	Hash string      `json:"hash"`
	Uses []globalUse `json:"uses"`
}

func readUses(root string, now listedWith) (map[string]sourceUses, error) {
	kept, found, err := readCache[keptUses](root, usesFile)
	if err != nil || !found || kept.listedWith != now || !kept.listsEverySource() {
		return map[string]sourceUses{}, err
	}
	return kept.Sources, nil
}

func (k keptUses) listsEverySource() bool {
	for _, source := range k.Sources {
		if source.Uses == nil || slices.ContainsFunc(source.Uses, func(use globalUse) bool { return use.Name == "" }) {
			return false
		}
	}
	return k.Sources != nil
}

func writeUses(root string, now listedWith, lists map[string]sourceUses) error {
	return writeCache(root, usesFile, keptUses{listedWith: now, Sources: lists})
}

func readCache[T any](root, name string) (kept T, found bool, err error) {
	var none T
	file, err := fsx.SafeJoinNoSymlinks(root, outputDir+"/"+name)
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

func writeCache(root, name string, kept any) error {
	path := outputDir + "/" + name
	text, err := json.MarshalIndent(kept, "", "  ")
	if err != nil {
		return fmt.Errorf("script: %s cannot be written as JSON: %w", path, err)
	}
	file, err := fsx.SafeJoinNoSymlinks(root, path)
	if err != nil {
		return err
	}
	if _, err := fsx.WriteIfChanged(file, string(text)+"\n"); err != nil {
		return errUnwritableOutput(path, err)
	}
	return nil
}
