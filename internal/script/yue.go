package script

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"slices"
	"strconv"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/env"
	"github.com/mdlsvensson/moonwell/internal/fsx"
)

const (
	outputDir = "dist/stage/lua"
	atOnce    = 8
)

type staged struct {
	texts        map[string]string
	hashes       map[string]string
	lua          map[string]string
	macroSources string
}

func compileAll(ctx context.Context, e *env.Env, yue string, minify bool, m macros, sources []Source) (*staged, error) {
	outputs, err := outputFolder(e.Root)
	if err != nil {
		return nil, err
	}
	units, err := unitsOf(e.Root, outputs, sources)
	if err != nil {
		return nil, err
	}
	now := dependsOn{Compiler: yue, Mode: modeOf(minify), Macros: m.hash, MacroSources: macroSourcesOf(units)}
	last, err := readHashes(e.Root)
	if err != nil {
		return nil, err
	}
	if err := removeGone(outputs, last, units); err != nil {
		return nil, err
	}
	stale, upToDate := last.stale(units, now)
	if err := writeHashes(e.Root, now, upToDate, stale); err != nil {
		return nil, err
	}
	with := compiler{ctx: ctx, run: e.Run, program: yue, mode: now.Mode, search: m.path}
	failures, err := with.compileEach(stale)
	if err != nil {
		return nil, err
	}
	if err := writeHashes(e.Root, now, withoutFailed(units, failures), nil); err != nil {
		return nil, err
	}
	if len(failures) > 0 {
		return nil, errNotCompiled(failures)
	}
	return stagedOf(units, now.MacroSources), nil
}

func (s *staged) luaOf(source Source) (lua string, ok bool, err error) {
	file, isCompiled := s.lua[source.Path]
	if source.Kind != Yue || !isCompiled {
		return "", false, nil
	}
	under, err := outputOf(source)
	if err != nil {
		return "", false, err
	}
	data, found, err := fsx.ReadIfThere(file)
	if err != nil {
		return "", false, errUnreadableOutput(outputDir+"/"+under, err)
	}
	return string(data), found, nil
}

func modeOf(minify bool) string {
	if minify {
		return "-m"
	}
	return "-r"
}

type unit struct {
	path   string
	file   string
	text   string
	hash   string
	under  string
	output string
}

func outputFolder(root string) (string, error) {
	return fsx.Inside(root, outputDir)
}

func unitsOf(root, outputs string, sources []Source) ([]unit, error) {
	var units []unit
	for _, source := range sources {
		if source.Kind != Yue {
			continue
		}
		u, err := unitOf(root, outputs, source)
		if err != nil {
			return nil, err
		}
		units = append(units, u)
	}
	return units, nil
}

func unitOf(root, outputs string, source Source) (unit, error) {
	under, err := outputOf(source)
	if err != nil {
		return unit{}, err
	}
	file := filepath.Join(root, filepath.FromSlash(source.Path))
	data, err := os.ReadFile(file)
	if err != nil {
		return unit{}, errUnreadableSource(source.Path, err)
	}
	return unit{
		path:   source.Path,
		file:   file,
		text:   fsx.WithoutMark(string(data)),
		hash:   fsx.SHA256Hex(data),
		under:  under,
		output: filepath.Join(outputs, filepath.FromSlash(under)),
	}, nil
}

var plainKey = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

func outputOf(source Source) (string, error) {
	below := strings.ReplaceAll(source.Name, ".", "/")
	switch {
	case source.Library == "" && source.Path == "src/"+below+".yue":
		return below + ".lua", nil
	case plainKey.MatchString(source.Library) && strings.HasSuffix(source.Path, "/"+below+".yue"):
		return ".libraries/" + source.Library + "/" + below + ".lua", nil
	}
	return "", fmt.Errorf("script.compileAll: the module %q of the library %q is at %q, which places no output", source.Name, source.Library, source.Path)
}

func withoutFailed(units []unit, failures []*diag.Error) []unit {
	failed := map[string]bool{}
	for _, failure := range failures {
		failed[failure.File] = true
	}
	return slices.DeleteFunc(slices.Clone(units), func(u unit) bool { return failed[u.path] })
}

func stagedOf(units []unit, macroSources string) *staged {
	result := &staged{
		texts: map[string]string{}, hashes: map[string]string{}, lua: map[string]string{}, macroSources: macroSources,
	}
	for _, u := range units {
		result.texts[u.path], result.hashes[u.path], result.lua[u.path] = u.text, u.hash, u.output
	}
	return result
}

type compiler struct {
	ctx     context.Context
	run     env.RunFunc
	program string
	mode    string
	search  string
}

func eachOf[T, G any](items []T, work func(T) (G, error)) (gave []G, err error) {
	type ended struct {
		at   int
		gave G
		err  error
	}
	gave = make([]G, len(items))
	over := make(chan ended)
	next, running := 0, 0
	for {
		if err == nil && next < len(items) && running < atOnce {
			go func(at int) {
				result, failed := guarded(work, items[at])
				over <- ended{at, result, failed}
			}(next)
			next, running = next+1, running+1
			continue
		}
		if running == 0 {
			return gave, err
		}
		done := <-over
		running--
		gave[done.at] = done.gave
		if done.err != nil && err == nil {
			err = done.err
		}
	}
}

func guarded[T, G any](work func(T) (G, error), item T) (gave G, err error) {
	defer func() {
		if fault := recover(); fault != nil {
			err = fmt.Errorf("%v\n%s", fault, debug.Stack())
		}
	}()
	return work(item)
}

func (c compiler) compileEach(units []unit) ([]*diag.Error, error) {
	refused, err := eachOf(units, c.compile)
	if err != nil {
		return nil, err
	}
	return slices.DeleteFunc(refused, func(failure *diag.Error) bool { return failure == nil }), nil
}

func (c compiler) compile(u unit) (refused *diag.Error, err error) {
	if err := os.MkdirAll(filepath.Dir(u.output), 0o777); err != nil {
		return nil, errUnwritableOutput(outputDir+"/"+u.under, err)
	}
	if err := removeOutput(u.under, u.output); err != nil {
		return nil, err
	}
	args := []string{"--target=5.3", c.mode, "-o", u.output, "--path", c.search, u.file}
	result, err := c.run(c.ctx, c.program, args, env.RunOptions{})
	if err != nil {
		return nil, err
	}
	failure := u.failureOf(result)
	if failure == nil {
		return nil, nil
	}
	if err := removeOutput(u.under, u.output); err != nil {
		return nil, err
	}
	return failure, nil
}

func removeOutput(under, file string) error {
	err := os.Remove(file)
	if err == nil || errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return errUnremovableOutput(outputDir+"/"+under, err)
}

func (u unit) failureOf(result env.RunResult) *diag.Error {
	if result.Code == 0 {
		if isEmptyFile(u.output) && hasCode(u.text) {
			return errEmptyOutput(u.path)
		}
		return nil
	}
	printed := result.Stdout + "\n" + result.Stderr
	if failure := rewriteError(u.path, printed, func() string { return leftAt(u.output) }); failure != nil {
		return failure
	}
	return compileError(u.path, printed)
}

func isEmptyFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular() && info.Size() == 0
}

func leftAt(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
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

func errEmptyOutput(file string) *diag.Error {
	return &diag.Error{
		Msg:  "YueScript reported success but wrote no Lua for " + file + ", although the file has code.",
		File: file,
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
