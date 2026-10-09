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

type compileCacheKey struct {
	Compiler     string `json:"compiler"`
	Mode         string `json:"mode"`
	Macros       string `json:"macros"`
	MacroSources string `json:"macroSources"`
}

type compileCache struct {
	compileCacheKey
	Sources map[string]cachedSource `json:"sources"`
}

type cachedSource struct {
	Hash   string `json:"hash"`
	Output string `json:"output"`
}

func readCompileCache(root string) (compileCache, error) {
	cache, found, err := readCacheFile[compileCache](root, hashesFile)
	if err != nil || !found || !cache.hasOutputs() {
		return compileCache{}, err
	}
	return cache, nil
}

func (c compileCache) hasOutputs() bool {
	for _, source := range c.Sources {
		if !filepath.IsLocal(filepath.FromSlash(source.Output)) || !strings.HasSuffix(source.Output, ".lua") {
			return false
		}
	}
	return true
}

func (c compileCache) splitStale(units []compileUnit, key compileCacheKey) (stale, upToDate []compileUnit) {
	for _, unit := range units {
		if c.compileCacheKey == key && c.Sources[unit.path] == (cachedSource{Hash: unit.hash, Output: unit.outputPath}) && fsx.Exists(unit.outputFullPath) {
			upToDate = append(upToDate, unit)
		} else {
			stale = append(stale, unit)
		}
	}
	return stale, upToDate
}

func removeStaleOutputs(outputRoot string, cache compileCache, units []compileUnit) error {
	currentOutputs := map[string]bool{}
	for _, unit := range units {
		currentOutputs[unit.outputPath] = true
	}
	for _, path := range slices.Sorted(maps.Keys(cache.Sources)) {
		outputPath := cache.Sources[path].Output
		if currentOutputs[outputPath] {
			continue
		}
		if err := removeOutput(outputPath, filepath.Join(outputRoot, filepath.FromSlash(outputPath))); err != nil {
			return err
		}
	}
	return nil
}

func writeCompileCache(root string, key compileCacheKey, compiled, pending []compileUnit) error {
	cache := compileCache{compileCacheKey: key, Sources: map[string]cachedSource{}}
	for _, unit := range compiled {
		cache.Sources[unit.path] = cachedSource{Hash: unit.hash, Output: unit.outputPath}
	}
	for _, unit := range pending {
		cache.Sources[unit.path] = cachedSource{Output: unit.outputPath}
	}
	return writeCacheFile(root, hashesFile, cache)
}

func macroSourcesOf(units []compileUnit) string {
	var withMacros []compileUnit
	for _, unit := range units {
		if containsMacroWord(unit.text) {
			withMacros = append(withMacros, unit)
		}
	}
	if len(withMacros) == 0 {
		return ""
	}
	slices.SortFunc(withMacros, func(a, b compileUnit) int { return strings.Compare(a.path, b.path) })
	var content strings.Builder
	for _, unit := range withMacros {
		content.WriteString(unit.path + "\x00" + unit.hash + "\n")
	}
	return fsx.SHA256Hex([]byte(content.String()))
}

func containsMacroWord(text string) bool {
	const word = "macro"
	for searchFrom := 0; ; {
		index := strings.Index(text[searchFrom:], word)
		if index < 0 {
			return false
		}
		start, end := searchFrom+index, searchFrom+index+len(word)
		if (start == 0 || !isWordByte(text[start-1])) && (end == len(text) || !isWordByte(text[end])) {
			return true
		}
		searchFrom = start + 1
	}
}

func isWordByte(b byte) bool {
	return b == '_' || (b >= '0' && b <= '9') || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

func readCacheFile[T any](root, name string) (cache T, found bool, err error) {
	var zero T
	fullPath, err := fsx.SafeJoinNoSymlinks(root, outputDir+"/"+name)
	if err != nil {
		return zero, false, err
	}
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return zero, false, nil
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&cache) != nil {
		return zero, false, nil
	}
	if _, err := decoder.Token(); err != io.EOF {
		return zero, false, nil
	}
	return cache, true, nil
}

func writeCacheFile(root, name string, cache any) error {
	path := outputDir + "/" + name
	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return fmt.Errorf("script: %s cannot be written as JSON: %w", path, err)
	}
	fullPath, err := fsx.SafeJoinNoSymlinks(root, path)
	if err != nil {
		return err
	}
	if _, err := fsx.WriteIfChanged(fullPath, string(data)+"\n"); err != nil {
		return errUnwritableOutput(path, err)
	}
	return nil
}
