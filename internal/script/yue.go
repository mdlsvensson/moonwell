package script

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/env"
	"github.com/mdlsvensson/moonwell/internal/fsx"
)

const outputDir = "dist/stage/lua"

func compileAll(ctx context.Context, e *env.Env, yue string, minify bool, macros macroFile, sources []Source) (*compileOutput, error) {
	outputRoot, err := stageLuaDir(e.Root)
	if err != nil {
		return nil, err
	}
	units, err := newCompileUnits(e.Root, outputRoot, sources)
	if err != nil {
		return nil, err
	}
	key := compileCacheKey{Compiler: yue, Mode: modeName(minify), Macros: macros.hash, MacroSources: macroSourcesOf(units)}
	cache, err := readCompileCache(e.Root)
	if err != nil {
		return nil, err
	}
	if err := removeStaleOutputs(outputRoot, cache, units); err != nil {
		return nil, err
	}
	stale, upToDate := cache.splitStale(units, key)
	if err := writeCompileCache(e.Root, key, upToDate, stale); err != nil {
		return nil, err
	}
	yueCompiler := compiler{ctx: ctx, run: e.Run, program: yue, mode: key.Mode, searchPath: macros.searchPath}
	failures, err := yueCompiler.compileEach(stale)
	if err != nil {
		return nil, err
	}
	if err := writeCompileCache(e.Root, key, withoutFailed(units, failures), nil); err != nil {
		return nil, err
	}
	if len(failures) > 0 {
		return nil, errNotCompiled(failures)
	}
	return newCompileOutput(units, key.MacroSources), nil
}

func stageLuaDir(root string) (string, error) {
	return fsx.SafeJoinNoSymlinks(root, outputDir)
}

type compileUnit struct {
	path           string
	fullPath       string
	text           string
	hash           string
	outputPath     string
	outputFullPath string
}

func newCompileUnits(root, outputRoot string, sources []Source) ([]compileUnit, error) {
	var units []compileUnit
	for _, source := range sources {
		if source.Kind != Yue {
			continue
		}
		unit, err := newCompileUnit(root, outputRoot, source)
		if err != nil {
			return nil, err
		}
		units = append(units, unit)
	}
	return units, nil
}

func newCompileUnit(root, outputRoot string, source Source) (compileUnit, error) {
	outputPath, err := luaPathOf(source)
	if err != nil {
		return compileUnit{}, err
	}
	fullPath := filepath.Join(root, filepath.FromSlash(source.Path))
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return compileUnit{}, errUnreadableSource(source.Path, err)
	}
	return compileUnit{
		path:           source.Path,
		fullPath:       fullPath,
		text:           fsx.TrimBOM(string(data)),
		hash:           fsx.SHA256Hex(data),
		outputPath:     outputPath,
		outputFullPath: filepath.Join(outputRoot, filepath.FromSlash(outputPath)),
	}, nil
}

var plainKey = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

func luaPathOf(source Source) (string, error) {
	modulePath := strings.ReplaceAll(source.Name, ".", "/")
	switch {
	case source.Library == "" && source.Path == "src/"+modulePath+".yue":
		return modulePath + ".lua", nil
	case plainKey.MatchString(source.Library) && strings.HasSuffix(source.Path, "/"+modulePath+".yue"):
		return ".libraries/" + source.Library + "/" + modulePath + ".lua", nil
	}
	return "", fmt.Errorf("script.compileAll: the module %q of the library %q is at %q, which places no output", source.Name, source.Library, source.Path)
}

func modeName(minify bool) string {
	if minify {
		return "-m"
	}
	return "-r"
}

type compiler struct {
	ctx        context.Context
	run        env.RunFunc
	program    string
	mode       string
	searchPath string
}

func (c compiler) compileEach(units []compileUnit) ([]*diag.Error, error) {
	diagErrs, err := runParallel(units, c.compile)
	if err != nil {
		return nil, err
	}
	return slices.DeleteFunc(diagErrs, func(diagErr *diag.Error) bool { return diagErr == nil }), nil
}

func (c compiler) compile(unit compileUnit) (*diag.Error, error) {
	if err := os.MkdirAll(filepath.Dir(unit.outputFullPath), 0o777); err != nil {
		return nil, errUnwritableOutput(outputDir+"/"+unit.outputPath, err)
	}
	if err := removeOutput(unit.outputPath, unit.outputFullPath); err != nil {
		return nil, err
	}
	args := []string{"--target=5.3", c.mode, "-o", unit.outputFullPath, "--path", c.searchPath, unit.fullPath}
	result, err := c.run(c.ctx, c.program, args, env.RunOptions{})
	if err != nil {
		return nil, err
	}
	diagErr := unit.diagErrorOf(result)
	if diagErr == nil {
		return nil, nil
	}
	if err := removeOutput(unit.outputPath, unit.outputFullPath); err != nil {
		return nil, err
	}
	return diagErr, nil
}

func removeOutput(outputPath, fullPath string) error {
	err := os.Remove(fullPath)
	if err == nil || errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return errUnremovableOutput(outputDir+"/"+outputPath, err)
}

func (unit compileUnit) diagErrorOf(result env.RunResult) *diag.Error {
	if result.ExitCode == 0 {
		if isEmptyFile(unit.outputFullPath) && hasCode(unit.text) {
			return errEmptyOutput(unit.path)
		}
		return nil
	}
	output := result.Stdout + "\n" + result.Stderr
	if diagErr := newRewriteError(unit.path, output, func() string { return readOrEmpty(unit.outputFullPath) }); diagErr != nil {
		return diagErr
	}
	return newCompileError(unit.path, output)
}

func isEmptyFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular() && info.Size() == 0
}

func readOrEmpty(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

func withoutFailed(units []compileUnit, failures []*diag.Error) []compileUnit {
	failed := map[string]bool{}
	for _, diagErr := range failures {
		failed[diagErr.File] = true
	}
	return slices.DeleteFunc(slices.Clone(units), func(unit compileUnit) bool { return failed[unit.path] })
}

type compileOutput struct {
	sourceTexts  map[string]string
	sourceHashes map[string]string
	outputFiles  map[string]string
	macroSources string
}

func newCompileOutput(units []compileUnit, macroSources string) *compileOutput {
	output := &compileOutput{
		sourceTexts: map[string]string{}, sourceHashes: map[string]string{}, outputFiles: map[string]string{}, macroSources: macroSources,
	}
	for _, unit := range units {
		output.sourceTexts[unit.path], output.sourceHashes[unit.path], output.outputFiles[unit.path] = unit.text, unit.hash, unit.outputFullPath
	}
	return output
}

func (o *compileOutput) readLua(source Source) (lua string, ok bool, err error) {
	outputFile, isCompiled := o.outputFiles[source.Path]
	if source.Kind != Yue || !isCompiled {
		return "", false, nil
	}
	outputPath, err := luaPathOf(source)
	if err != nil {
		return "", false, err
	}
	data, found, err := fsx.ReadFileIfExists(outputFile)
	if err != nil {
		return "", false, errUnreadableOutput(outputDir+"/"+outputPath, err)
	}
	return string(data), found, nil
}

func errUnreadableSource(path string, cause error) error {
	return &diag.Error{
		Msg:   "Reading " + path + " failed: " + fsx.Reason(cause),
		File:  path,
		Hint:  "Close any program that has the file open and check that it is a readable file, then try again.",
		Cause: cause,
	}
}

const distHint = "Moonwell writes dist/ itself: close any program that has the file open, or delete dist/, then try again."

func errUnreadableOutput(path string, cause error) error {
	return &diag.Error{Msg: "Reading " + path + " failed: " + fsx.Reason(cause), File: path, Hint: distHint, Cause: cause}
}

func errUnwritableOutput(path string, cause error) error {
	return &diag.Error{Msg: "Writing " + path + " failed: " + fsx.Reason(cause), File: path, Hint: distHint, Cause: cause}
}

func errUnremovableOutput(path string, cause error) error {
	return &diag.Error{Msg: "Removing " + path + " failed: " + fsx.Reason(cause), File: path, Hint: distHint, Cause: cause}
}

func errEmptyOutput(path string) *diag.Error {
	return &diag.Error{
		Msg:  "YueScript reported success but wrote no Lua for " + path + ", although the file has code.",
		File: path,
		Hint: "YueScript 0.34.2 does this for a file that uses the floor division operator `//` or a bitwise operator. " +
			"Use YueScript 0.34.3 (the default from Moonwell 0.8.1 on), or write math.floor(a / b) instead.",
	}
}

func firstByPath(failures []*diag.Error) *diag.Error {
	return slices.MinFunc(failures, func(a, b *diag.Error) int { return strings.Compare(a.File, b.File) })
}

func errNotCompiled(failures []*diag.Error) error {
	first := firstByPath(failures)
	if len(failures) == 1 {
		return first
	}
	return &diag.Error{
		Msg:  first.Msg + "\n(" + strconv.Itoa(len(failures)-1) + " more file(s) failed to compile)",
		File: first.File,
		Line: first.Line,
	}
}
