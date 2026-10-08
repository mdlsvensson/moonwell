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

func macroSourcesOf(units []compileUnit) string {
	var holding []compileUnit
	for _, u := range units {
		if containsMacroWord(u.text) {
			holding = append(holding, u)
		}
	}
	if len(holding) == 0 {
		return ""
	}
	slices.SortFunc(holding, func(a, b compileUnit) int { return strings.Compare(a.path, b.path) })
	var hashed strings.Builder
	for _, u := range holding {
		hashed.WriteString(u.path + "\x00" + u.hash + "\n")
	}
	return fsx.SHA256Hex([]byte(hashed.String()))
}

func containsMacroWord(text string) bool {
	const word = "macro"
	for from := 0; ; {
		at := strings.Index(text[from:], word)
		if at < 0 {
			return false
		}
		start, end := from+at, from+at+len(word)
		if (start == 0 || !isWordByte(text[start-1])) && (end == len(text) || !isWordByte(text[end])) {
			return true
		}
		from = start + 1
	}
}

func isWordByte(b byte) bool {
	return b == '_' || (b >= '0' && b <= '9') || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

type compileCache struct {
	dependsOn
	Sources map[string]cachedSource `json:"sources"`
}

type cachedSource struct {
	Hash   string `json:"hash"`
	Output string `json:"output"`
}

func readCompileCache(root string) (compileCache, error) {
	kept, found, err := readCacheFile[compileCache](root, hashesFile)
	if err != nil || !found || !kept.hasOutputs() {
		return compileCache{}, err
	}
	return kept, nil
}

func (h compileCache) hasOutputs() bool {
	for _, source := range h.Sources {
		if !filepath.IsLocal(filepath.FromSlash(source.Output)) || !strings.HasSuffix(source.Output, ".lua") {
			return false
		}
	}
	return true
}

func (h compileCache) splitStale(units []compileUnit, now dependsOn) (stale, upToDate []compileUnit) {
	for _, u := range units {
		if h.dependsOn == now && h.Sources[u.path] == (cachedSource{Hash: u.hash, Output: u.outputDir}) && fsx.Exists(u.output) {
			upToDate = append(upToDate, u)
		} else {
			stale = append(stale, u)
		}
	}
	return stale, upToDate
}

func removeStaleOutputs(outputs string, last compileCache, units []compileUnit) error {
	current := map[string]bool{}
	for _, u := range units {
		current[u.outputDir] = true
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

func writeCompileCache(root string, now dependsOn, good, pending []compileUnit) error {
	kept := compileCache{dependsOn: now, Sources: map[string]cachedSource{}}
	for _, u := range good {
		kept.Sources[u.path] = cachedSource{Hash: u.hash, Output: u.outputDir}
	}
	for _, u := range pending {
		kept.Sources[u.path] = cachedSource{Output: u.outputDir}
	}
	return writeCacheFile(root, hashesFile, kept)
}

const usesFile = ".globals.json"

type listedWith struct {
	Compiler     string `json:"compiler"`
	Macros       string `json:"macros"`
	MacroSources string `json:"macroSources"`
}

type usesCache struct {
	listedWith
	Sources map[string]cachedUses `json:"sources"`
}

type cachedUses struct {
	Hash string      `json:"hash"`
	Uses []globalUse `json:"uses"`
}

func readUsesCache(root string, now listedWith) (map[string]cachedUses, error) {
	kept, found, err := readCacheFile[usesCache](root, usesFile)
	if err != nil || !found || kept.listedWith != now || !kept.listsEverySource() {
		return map[string]cachedUses{}, err
	}
	return kept.Sources, nil
}

func (k usesCache) listsEverySource() bool {
	for _, source := range k.Sources {
		if source.Uses == nil || slices.ContainsFunc(source.Uses, func(use globalUse) bool { return use.Name == "" }) {
			return false
		}
	}
	return k.Sources != nil
}

func writeUsesCache(root string, now listedWith, lists map[string]cachedUses) error {
	return writeCacheFile(root, usesFile, usesCache{listedWith: now, Sources: lists})
}

func readCacheFile[T any](root, name string) (kept T, found bool, err error) {
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

func writeCacheFile(root, name string, kept any) error {
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
