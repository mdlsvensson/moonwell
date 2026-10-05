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

	"github.com/mdlsvensson/moonwell/next/internal/diag"
	"github.com/mdlsvensson/moonwell/next/internal/env"
	"github.com/mdlsvensson/moonwell/next/internal/fsx"
)

const (
	// stageDir is where a compile leaves its Lua, and the files it keeps beside it, from the project folder.
	stageDir = "dist/stage/lua"
	// atOnce is how many compilers run at a time.
	atOnce = 8
	// byteOrderMark is U+FEFF in UTF-8.
	byteOrderMark = "\xEF\xBB\xBF"
)

// compiled is what a compile left in dist/stage/lua.
type compiled struct {
	texts  map[string]string // each YueScript source's text, by its path from the project folder
	hashes map[string]string // the SHA-256 of each source's bytes, by the same path
	lua    map[string]string // where each compiled file is, by the same path
}

// compileAll compiles every YueScript module into dist/stage/lua, at most eight at a time, and recompiles only
// the files that changed since the last run.
//
// A file the compiler refuses does not stop the others: the failure returned is that of the first such file by
// its path, with the count of the others. Any other failure (a file that cannot be read or written, a compiler
// that cannot be started, a cancelled context) is returned as it is, and no further compiler is started.
//
// What a compile leaves depends on nothing an earlier run left. The output of a source is removed before the
// compiler runs on it, and before the first compiler runs, the hashes file stops vouching for every source that
// is to be compiled: a run that is stopped leaves nothing up to date that it may have touched.
func compileAll(ctx context.Context, e *env.Env, yue string, minify bool, m macros, sources []Source) (*compiled, error) {
	stage, err := stageOf(e.Root)
	if err != nil {
		return nil, err
	}
	units, err := unitsOf(e.Root, stage, sources)
	if err != nil {
		return nil, err
	}
	now := dependsOn{Compiler: yue, Mode: modeOf(minify), Macros: m.hash}
	last, err := readHashes(e.Root)
	if err != nil {
		return nil, err
	}
	if err := removeGone(stage, last, units); err != nil {
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
	return compiledOf(units), nil
}

// luaOf reads a compiled module; ok is false when it has no output. The Lua is the bytes the compiler wrote:
// nothing is decoded. A module that is no YueScript, or that the compile was not given, has no output, and
// neither has a source without code, for which the compiler writes no file.
func (c *compiled) luaOf(source Source) (lua string, ok bool, err error) {
	file, isCompiled := c.lua[source.Path]
	if source.Kind != Yue || !isCompiled {
		return "", false, nil
	}
	under, err := outputOf(source)
	if err != nil {
		return "", false, err
	}
	data, err := os.ReadFile(file)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return "", false, nil
	case err != nil:
		return "", false, errUnreadableOutput(stageDir+"/"+under, err)
	}
	return string(data), true, nil
}

// modeOf is the compiler's flag for the Lua it writes: -r keeps each statement on the line of its source, and
// -m minifies.
func modeOf(minify bool) string {
	if minify {
		return "-m"
	}
	return "-r"
}

// ---- the sources ----

// unit is a YueScript source with what a compile of it needs.
type unit struct {
	path   string // the source, from the project folder, with "/"
	file   string // the source on disk
	text   string // the source's bytes, without a byte order mark at the start
	hash   string // the SHA-256 of the source's bytes
	under  string // where its Lua goes, from stageDir, with "/"
	output string // the same place on disk
}

// stageOf is the staging folder of the project at root, on disk. A link at the folder, or on the way to it, is
// refused. Below the folder nothing is looked at for links: it is Moonwell's own, which Moonwell makes and
// fills, so a link that is planted in it is written through.
func stageOf(root string) (string, error) {
	return placeOf(root, stageDir, errUnwritableOutput)
}

// unitsOf reads every YueScript source among the modules, in their order; stage is the staging folder on disk.
func unitsOf(root, stage string, sources []Source) ([]unit, error) {
	var units []unit
	for _, source := range sources {
		if source.Kind != Yue {
			continue
		}
		u, err := unitOf(root, stage, source)
		if err != nil {
			return nil, err
		}
		units = append(units, u)
	}
	return units, nil
}

// unitOf reads a YueScript source and finds where its Lua goes. The source is read where the listing of the
// modules found it: at its path below the project folder, whatever the file is called, and through a link in
// its place. Nothing of the text is decoded, so it may hold bytes that are not UTF-8.
func unitOf(root, stage string, source Source) (unit, error) {
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
		text:   strings.TrimPrefix(string(data), byteOrderMark),
		hash:   fsx.SHA256Hex(data),
		under:  under,
		output: filepath.Join(stage, filepath.FromSlash(under)),
	}, nil
}

// plainKey matches a library's key as a manifest spells one: one name of letters, digits, "_" and "-".
var plainKey = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// outputOf is where a YueScript source compiles to, from stageDir: src/x.yue to x.lua, and a library's x.yue to
// .libraries/<key>/x.lua. A module's name is its path below its folder, with a dot for each "/", so the name
// and the library's key place the output, whatever folder the library is in. No step of that path is "." or
// "..": a name's dots are all separators, and a key is one plain name.
func outputOf(source Source) (string, error) {
	below := strings.ReplaceAll(source.Name, ".", "/")
	switch {
	case source.Library == "" && source.Path == "src/"+below+".yue":
		return below + ".lua", nil
	case plainKey.MatchString(source.Library) && strings.HasSuffix(source.Path, "/"+below+".yue"):
		return ".libraries/" + source.Library + "/" + below + ".lua", nil
	}
	// A plain error: the listing of the modules names every module by its path, and the libraries' keys are a
	// manifest's, which are plain names. So a YueScript module that is not at the path of its name, or a key that
	// is no plain name, is a mistake in Moonwell and nothing the user can put right.
	return "", fmt.Errorf("script.compileAll: the module %q of the library %q is at %q, which places no output", source.Name, source.Library, source.Path)
}

// placeOf is where a file or a folder of the project is on disk; path is from the project folder, with "/". A
// link at it, or on the way to it, is refused. Any other failure to look at the place is worded by failed.
func placeOf(root, path string, failed func(path string, cause error) error) (string, error) {
	onDisk, err := fsx.SafeJoin(root, path)
	var refused *diag.Error
	if err != nil && !errors.As(err, &refused) {
		return "", failed(path, err)
	}
	return onDisk, err
}

// withoutFailed is the units whose compile left their Lua: all but those a failure names.
func withoutFailed(units []unit, failures []*diag.Error) []unit {
	failed := map[string]bool{}
	for _, failure := range failures {
		failed[failure.File] = true
	}
	return slices.DeleteFunc(slices.Clone(units), func(u unit) bool { return failed[u.path] })
}

// compiledOf is what a compile without failures left.
func compiledOf(units []unit) *compiled {
	result := &compiled{texts: map[string]string{}, hashes: map[string]string{}, lua: map[string]string{}}
	for _, u := range units {
		result.texts[u.path], result.hashes[u.path], result.lua[u.path] = u.text, u.hash, u.output
	}
	return result
}

// ---- running the compiler ----

// compiler is how the compiler is run on the sources of one project: to compile them, and to list the globals
// they use.
type compiler struct {
	ctx     context.Context
	run     env.RunFunc
	program string // the compiler's path
	mode    string // -r or -m, for a compile
	search  string // the --path that finds the macro module
}

// eachOf does the work on every item, at most atOnce at a time, and returns what each gave, in the order of the
// items. After an error no further work is started, the work that runs is waited for, and the first error is
// returned.
//
// Only this function's own goroutine reads and writes what it counts and gathers; the work on an item hands
// over what it gave on the channel and touches nothing else that is shared. The function returns when all the
// work it started has handed over, so none outlives it.
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
				result, failed := work(items[at])
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

// compileEach compiles the units, at most atOnce at a time, and returns the failures of the files the compiler
// refused. Any other error stops it, as eachOf says.
func (c compiler) compileEach(units []unit) ([]*diag.Error, error) {
	refused, err := eachOf(units, c.compile)
	if err != nil {
		return nil, err
	}
	return slices.DeleteFunc(refused, func(failure *diag.Error) bool { return failure == nil }), nil
}

// compile runs the compiler on one source, with no file at the source's output, and returns the failure of a
// file the compiler refused: nil for a source it took. With a file at the output, the compiler rewrites or
// minifies that file where the source has no code, and with none it writes none: so the output of an earlier
// run is removed first, and what is at the output afterwards is what this run wrote. The folder is made before
// that, so a file in the folder's place is refused in the same words on every system.
//
// A source the compiler refused has no file at its output afterwards either: the compiler leaves the Lua it
// could not rewrite.
func (c compiler) compile(u unit) (refused *diag.Error, err error) {
	if err := os.MkdirAll(filepath.Dir(u.output), 0o777); err != nil {
		return nil, errUnwritableOutput(stageDir+"/"+u.under, err)
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

// removeOutput removes a file of the staging folder: under is its path from there, with "/", and file its place
// on disk. A file that is not there is no failure. Every other failure is the output's own, whatever the
// system's reason: a file that another program holds is named from the project folder as any other is.
func removeOutput(under, file string) error {
	err := os.Remove(file)
	if err == nil || errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return errUnremovableOutput(stageDir+"/"+under, err)
}

// failureOf is the failure of a run of the compiler on the unit; nil for a run that went well. What the run
// printed is read in printed.go.
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

// isEmptyFile reports whether there is a file without a byte at path.
func isEmptyFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular() && info.Size() == 0
}

// leftAt is the text of the file at path; "" when it cannot be read.
func leftAt(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

// ---- errors ----

func errUnreadableSource(path string, cause error) error {
	return &diag.Error{
		Msg:   "Reading " + path + " failed: " + fsx.Reason(cause),
		File:  path,
		Hint:  "Close any program that has the file open and check that it is a readable file, then try again.",
		Cause: cause,
	}
}

// stageHint ends a failure to read, write or remove a file below dist/.
const stageHint = "Moonwell writes dist/ itself: close any program that has the file open, or delete dist/, then try again."

func errUnreadableOutput(path string, cause error) error {
	return &diag.Error{Msg: "Reading " + path + " failed: " + fsx.Reason(cause), File: path, Hint: stageHint, Cause: cause}
}

func errUnwritableOutput(path string, cause error) error {
	return &diag.Error{Msg: "Writing " + path + " failed: " + fsx.Reason(cause), File: path, Hint: stageHint, Cause: cause}
}

func errUnremovableOutput(path string, cause error) error {
	return &diag.Error{Msg: "Removing " + path + " failed: " + fsx.Reason(cause), File: path, Hint: stageHint, Cause: cause}
}

// errEmptyOutput is the failure of a compile that reported success and wrote an empty file for a source that
// has code. YueScript 0.34.2 does that for a source that uses floor division (`//`) or a bitwise operator, with
// both -r and -m, and the module would silently be missing from the build.
func errEmptyOutput(file string) *diag.Error {
	return &diag.Error{
		Msg:  "YueScript reported success but wrote no Lua for " + file + ", although the file has code.",
		File: file,
		Hint: "YueScript 0.34.2 does this for a file that uses the floor division operator `//` or a bitwise operator. " +
			"Use YueScript 0.34.3 (the default from Moonwell 0.8.1 on), or write math.floor(a / b) instead.",
	}
}

// firstByPath is the failure of the file that is first by the bytes of its path.
func firstByPath(failures []*diag.Error) *diag.Error {
	return slices.MinFunc(failures, func(a, b *diag.Error) int { return strings.Compare(a.File, b.File) })
}

// errNotCompiled is the failure of a compile in which the compiler refused files: that of the first file by the
// bytes of its path and, when there are more, the count of the others in place of its hint.
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
