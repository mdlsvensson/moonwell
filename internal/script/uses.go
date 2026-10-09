package script

import (
	"context"
	"maps"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/env"
	"github.com/mdlsvensson/moonwell/internal/fsx"
)

type globalUse struct {
	Name   string `json:"name"`
	Line   int    `json:"line"`
	Column int    `json:"column"`
}

type lintSource struct {
	path string
	hash string
}

func lintSources(modules []Module, output *compileOutput) []lintSource {
	byPath := map[string]lintSource{}
	for _, module := range modules {
		if hash, ok := output.sourceHashes[module.Path]; ok && module.Kind == Yue && module.Library == "" {
			byPath[module.Path] = lintSource{path: module.Path, hash: hash}
		}
	}
	sources := make([]lintSource, 0, len(byPath))
	for _, path := range slices.Sorted(maps.Keys(byPath)) {
		sources = append(sources, byPath[path])
	}
	return sources
}

type lintResult struct {
	source  lintSource
	uses    []globalUse
	diagErr *diag.Error
}

func listGlobalUses(ctx context.Context, e *env.Env, yue string, macros macroFile, macroSources string, sources []lintSource) (map[string][]globalUse, error) {
	key := usesCacheKey{Compiler: yue, Macros: macros.hash, MacroSources: macroSources}
	cache, err := readUsesCache(e.Root, key)
	if err != nil {
		return nil, err
	}
	uses, stale := splitCachedUses(cache, sources)
	yueCompiler := compiler{ctx: ctx, run: e.Run, program: yue, searchPath: macros.searchPath}
	results, err := runParallel(stale, func(source lintSource) (lintResult, error) { return yueCompiler.listUses(e.Root, source) })
	if err != nil {
		return nil, err
	}
	var failures []*diag.Error
	for _, result := range results {
		if result.diagErr != nil {
			failures = append(failures, result.diagErr)
		} else {
			uses[result.source.path] = cachedUses{Hash: result.source.hash, Uses: result.uses}
		}
	}
	if err := writeUsesCache(e.Root, key, uses); err != nil {
		return nil, err
	}
	if len(failures) > 0 {
		return nil, firstByPath(failures)
	}
	return flattenUses(uses), nil
}

func splitCachedUses(cache map[string]cachedUses, sources []lintSource) (uses map[string]cachedUses, stale []lintSource) {
	uses = map[string]cachedUses{}
	for _, source := range sources {
		if cached, found := cache[source.path]; found && cached.Hash == source.hash {
			uses[source.path] = cached
		} else {
			stale = append(stale, source)
		}
	}
	return uses, stale
}

func flattenUses(cached map[string]cachedUses) map[string][]globalUse {
	uses := make(map[string][]globalUse, len(cached))
	for path, entry := range cached {
		uses[path] = entry.Uses
	}
	return uses
}

func (c compiler) listUses(root string, source lintSource) (lintResult, error) {
	fullPath := filepath.Join(root, filepath.FromSlash(source.path))
	result, err := c.run(c.ctx, c.program, []string{"-g", "--path", c.searchPath, fullPath}, env.RunOptions{})
	if err != nil {
		return lintResult{}, err
	}
	if result.ExitCode != 0 {
		return lintResult{source: source, diagErr: newCompileError(source.path, result.Stdout+"\n"+result.Stderr)}, nil
	}
	uses, diagErr := parseGlobalUses(result.Stdout, source.path)
	return lintResult{source: source, uses: uses, diagErr: diagErr}, nil
}

var useLine = regexp.MustCompile(`^([^` + fsx.ASCIISpace + `]+) ([0-9]+) ([0-9]+)$`)

func parseGlobalUses(output, path string) ([]globalUse, *diag.Error) {
	uses := []globalUse{}
	for _, rawLine := range lineEnd.Split(output, -1) {
		line := fsx.TrimASCIISpace(rawLine)
		if line == "" {
			continue
		}
		use := useLine.FindStringSubmatch(line)
		if use == nil {
			return nil, errUnreadableUse(path, line)
		}
		lineNumber, _ := strconv.Atoi(use[2])
		column, _ := strconv.Atoi(use[3])
		uses = append(uses, globalUse{Name: use[1], Line: lineNumber, Column: column})
	}
	return uses, nil
}

const usesFile = ".globals.json"

type usesCacheKey struct {
	Compiler     string `json:"compiler"`
	Macros       string `json:"macros"`
	MacroSources string `json:"macroSources"`
}

type usesCache struct {
	usesCacheKey
	Sources map[string]cachedUses `json:"sources"`
}

type cachedUses struct {
	Hash string      `json:"hash"`
	Uses []globalUse `json:"uses"`
}

func readUsesCache(root string, key usesCacheKey) (map[string]cachedUses, error) {
	cache, found, err := readCacheFile[usesCache](root, usesFile)
	if err != nil || !found || cache.usesCacheKey != key || !cache.listsEverySource() {
		return map[string]cachedUses{}, err
	}
	return cache.Sources, nil
}

func (c usesCache) listsEverySource() bool {
	for _, source := range c.Sources {
		if source.Uses == nil || slices.ContainsFunc(source.Uses, func(use globalUse) bool { return use.Name == "" }) {
			return false
		}
	}
	return c.Sources != nil
}

func writeUsesCache(root string, key usesCacheKey, uses map[string]cachedUses) error {
	return writeCacheFile(root, usesFile, usesCache{usesCacheKey: key, Sources: uses})
}

func errUnreadableUse(path, line string) *diag.Error {
	return &diag.Error{
		Msg:  "yue -g printed a line Moonwell cannot read: " + line,
		File: path,
		Hint: "Use a YueScript version Moonwell supports: remove yue.version from moonwell.toml and yue.path from the config.toml of your Moonwell folder.",
	}
}
