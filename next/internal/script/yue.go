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
	// luaSpace is the white space of Lua, which is that of YueScript and of what the compiler prints.
	luaSpace = " \t\n\v\f\r"
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
func compileAll(ctx context.Context, e *env.Env, yue string, minify bool, m macros, sources []Source) (*compiled, error) {
	units, err := unitsOf(e.Root, sources)
	if err != nil {
		return nil, err
	}
	now := dependsOn{Compiler: yue, Mode: modeOf(minify), Macros: m.hash}
	last, err := readHashes(e.Root)
	if err != nil {
		return nil, err
	}
	if err := removeGone(e.Root, last, units); err != nil {
		return nil, err
	}
	with := compiler{ctx: ctx, run: e.Run, program: yue, mode: now.Mode, search: m.path}
	failures, err := with.compileEach(last.stale(units, now))
	if err != nil {
		return nil, err
	}
	if err := writeHashes(e.Root, now, withoutFailed(units, failures)); err != nil {
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

// unitsOf reads every YueScript source among the modules, in their order.
func unitsOf(root string, sources []Source) ([]unit, error) {
	var units []unit
	for _, source := range sources {
		if source.Kind != Yue {
			continue
		}
		u, err := unitOf(root, source)
		if err != nil {
			return nil, err
		}
		units = append(units, u)
	}
	return units, nil
}

// unitOf reads a YueScript source and finds where its Lua goes. Nothing of the text is decoded, so it may hold
// bytes that are not UTF-8. A link at the source, at its output or on the way to either is refused.
func unitOf(root string, source Source) (unit, error) {
	under, err := outputOf(source)
	if err != nil {
		return unit{}, err
	}
	file, err := placeOf(root, source.Path, errUnreadableSource)
	if err != nil {
		return unit{}, err
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return unit{}, errUnreadableSource(source.Path, err)
	}
	output, err := placeOf(root, stageDir+"/"+under, errUnwritableOutput)
	if err != nil {
		return unit{}, err
	}
	text := strings.TrimPrefix(string(data), byteOrderMark)
	return unit{path: source.Path, file: file, text: text, hash: fsx.SHA256Hex(data), under: under, output: output}, nil
}

// outputOf is where a YueScript source compiles to, from stageDir: src/x.yue to x.lua, and a library's x.yue to
// .libraries/<key>/x.lua. A module's name is its path below its folder, with a dot for each "/", so the name
// and the library's key place the output, whatever folder the library is in.
func outputOf(source Source) (string, error) {
	below := strings.ReplaceAll(source.Name, ".", "/")
	inProject := source.Library == "" && source.Path == "src/"+below+".yue"
	inLibrary := source.Library != "" && strings.HasSuffix(source.Path, "/"+below+".yue")
	switch {
	case source.Name == "" || (!inProject && !inLibrary):
		// A plain error: Collect names every module by its path, so a YueScript module that is not at the path
		// of its name is a mistake in Moonwell and nothing the user can put right.
		return "", fmt.Errorf("script.compileAll: the module %q is at %q, where no YueScript module of that name is", source.Name, source.Path)
	case inLibrary:
		return ".libraries/" + source.Library + "/" + below + ".lua", nil
	}
	return below + ".lua", nil
}

// placeOf is where a file of the project is on disk; path is from the project folder, with "/". A link at the
// file, or on the way to it, is refused. Any other failure to look at the place is worded by failed.
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

// compiler is how the sources of one compile are compiled.
type compiler struct {
	ctx     context.Context
	run     env.RunFunc
	program string // the compiler's path
	mode    string // -r or -m
	search  string // the --path that finds the macro module
}

// outcome is how the compile of one source ended: failure for a file the compiler refused, err for anything
// else that went wrong, and neither for a source whose Lua is at its output.
type outcome struct {
	failure *diag.Error
	err     error
}

// compileEach compiles the units, at most atOnce at a time, and returns the failures of the files the compiler
// refused. After any other error no further compiler is started, the ones that run are waited for, and the
// first such error is returned.
//
// Only this function's own goroutine reads and writes what it counts and gathers; a compile hands its outcome
// over the channel and touches nothing else that is shared. The function returns when every compile it started
// has handed over its outcome, so none outlives it.
func (c compiler) compileEach(units []unit) (failures []*diag.Error, err error) {
	outcomes := make(chan outcome)
	next, running := 0, 0
	for {
		if err == nil && next < len(units) && running < atOnce {
			go func(u unit) { outcomes <- c.compile(u) }(units[next])
			next, running = next+1, running+1
			continue
		}
		if running == 0 {
			return failures, err
		}
		done := <-outcomes
		running--
		switch {
		case done.err != nil && err == nil:
			err = done.err
		case done.failure != nil:
			failures = append(failures, done.failure)
		}
	}
}

// compile runs the compiler on one source. A source the compiler refused has no file at its output afterwards,
// so nothing takes Lua of another text, or Lua that could not be rewritten, for its own.
func (c compiler) compile(u unit) outcome {
	if err := os.MkdirAll(filepath.Dir(u.output), 0o777); err != nil {
		return outcome{err: errUnwritableOutput(stageDir+"/"+u.under, err)}
	}
	args := []string{"--target=5.3", c.mode, "-o", u.output, "--path", c.search, u.file}
	result, err := c.run(c.ctx, c.program, args, env.RunOptions{})
	if err != nil {
		return outcome{err: err}
	}
	failure := u.failureOf(result)
	if failure == nil {
		return outcome{}
	}
	if err := removeOutput(u.under, u.output); err != nil {
		return outcome{err: err}
	}
	return outcome{failure: failure}
}

// removeOutput removes a file of the staging folder: under is its path from there, with "/", and file its place
// on disk. A file that is not there is no failure.
func removeOutput(under, file string) error {
	err := fsx.RemoveFile(file)
	var expected *diag.Error
	if err != nil && !errors.As(err, &expected) {
		return errUnremovableOutput(stageDir+"/"+under, err)
	}
	return err
}

// ---- reading what the compiler prints ----

// failureOf is the failure of a run of the compiler on the unit; nil for a run that went well.
func (u unit) failureOf(result env.RunResult) *diag.Error {
	if result.Code == 0 {
		if isEmptyFile(u.output) && hasCode(u.text) {
			return errEmptyOutput(u.path)
		}
		return nil
	}
	printed := result.Stdout + "\n" + result.Stderr
	if failure := rewriteError(u.path, printed, leftAt(u.output)); failure != nil {
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

var blankOrComment = regexp.MustCompile(`^[ \t\n\v\f\r]*(--[^\n\r]*)?$`)

// hasCode reports whether a source has a line that is neither blank nor a comment. The compiler rightly writes
// no Lua for a source without one. A line ends at "\n" or "\r\n".
func hasCode(source string) bool {
	for line := range strings.SplitSeq(strings.ReplaceAll(source, "\r\n", "\n"), "\n") {
		if !blankOrComment.MatchString(line) {
			return true
		}
	}
	return false
}

var (
	rewriteFailure = regexp.MustCompile(`(?m)^Failed to (rewrite|minify): `)
	rewriteReason  = regexp.MustCompile(`(?m)^>> :([0-9]+):[0-9]+: ([^\n\r]+)`)
	lineMark       = regexp.MustCompile(` -- ([0-9]+)$`)
)

// rewriteError is the failure of a compile whose YueScript was fine but whose Lua the compiler could not
// rewrite (-r) or minify (-m): that step does not read Lua 5.3's bitwise operators. The compiler prints "Failed
// to rewrite: <output>" and ">> :<line>:<column>: <reason>", a position in the Lua it leaves at the output,
// which left is. In a normal build each line of that Lua ends with its source line as a comment, which gives
// the error its line; minified Lua has no such marks. It returns nil for any other failure.
func rewriteError(file, printed, left string) *diag.Error {
	step := rewriteFailure.FindStringSubmatch(printed)
	if step == nil {
		return nil
	}
	reason := rewriteReason.FindStringSubmatch(printed)
	if reason == nil {
		return errNotRewritten(file, step[1], ".", 0)
	}
	// A position beyond what a number holds is beyond the Lua too.
	at, _ := strconv.Atoi(reason[1])
	return errNotRewritten(file, step[1], ": "+strings.Trim(reason[2], luaSpace), markedLine(left, at))
}

// markedLine is the source line that line at of rewritten Lua is marked with, counted from 1; 0 for a line that
// is not there or carries no mark.
func markedLine(lua string, at int) int {
	lines := strings.Split(strings.ReplaceAll(lua, "\r\n", "\n"), "\n")
	if at < 1 || at > len(lines) {
		return 0
	}
	mark := lineMark.FindStringSubmatch(lines[at-1])
	if mark == nil {
		return 0
	}
	line, _ := strconv.Atoi(mark[1])
	return line
}

var (
	lineEnd = regexp.MustCompile(`\r?\n`)
	// numberedLine finds "<line>: <message>" at the start of a line, where a carriage return alone starts a line
	// too: the compiler's excerpt of a source keeps the ones the source has.
	numberedLine  = regexp.MustCompile(`(?:\A|[\n\r])([0-9]+): ([^\n\r]+)`)
	macroPosition = regexp.MustCompile(`^failed to expand macro: \(macro [^)]*\):[0-9]+: `)
)

// compileError turns what the compiler printed for a failed file into an error at the file's line. The compiler
// prints "Failed to compile: <file>", then "<line>: <message>" and an excerpt of the source. A failing macro's
// message starts with "failed to expand macro: (macro <name>):<line>: ", a line of the macro module and not of
// the file, so the first line of the error drops it; the excerpt keeps the compiler's full text.
func compileError(file, printed string) *diag.Error {
	var kept []string
	for _, line := range lineEnd.Split(printed, -1) {
		if !strings.HasPrefix(line, "Failed to compile") {
			kept = append(kept, line)
		}
	}
	detail := strings.Trim(strings.Join(kept, "\n"), luaSpace)
	numbered := numberedLine.FindStringSubmatch(printed)
	switch {
	case numbered != nil:
		// A line beyond what a number holds is the greatest number: it stays a line the file has not.
		line, _ := strconv.Atoi(numbered[1])
		return errRefused(file, macroPosition.ReplaceAllString(numbered[2], "")+"\n"+detail, line)
	case detail == "":
		return errRefused(file, "YueScript compilation failed.", 0)
	}
	return errRefused(file, detail, 0)
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

// errRefused is the failure of a file the compiler refused, in the compiler's words.
func errRefused(file, printed string, line int) *diag.Error {
	return &diag.Error{Msg: printed, File: file, Line: line}
}

// errNotRewritten is the failure of a file whose Lua the compiler could not rewrite or minify, which step
// names. ending is the full stop, or the compiler's reason after a colon.
func errNotRewritten(file, step, ending string, line int) *diag.Error {
	return &diag.Error{
		Msg:  "YueScript compiled this file but could not " + step + " its Lua" + ending,
		File: file,
		Line: line,
		Hint: "That step of YueScript does not read Lua 5.3's bitwise operators (&, |, ~, <<, >>). If the file uses one, " +
			"move that code to a Lua module under lua/, which is bundled as written.",
	}
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

// errNotCompiled is the failure of a compile in which the compiler refused files: that of the first file by the
// bytes of its path and, when there are more, the count of the others in place of its hint.
func errNotCompiled(failures []*diag.Error) error {
	first := slices.MinFunc(failures, func(a, b *diag.Error) int { return strings.Compare(a.File, b.File) })
	if len(failures) == 1 {
		return first
	}
	return &diag.Error{
		Msg:  first.Msg + "\n(" + strconv.Itoa(len(failures)-1) + " more file(s) failed to compile)",
		File: first.File,
		Line: first.Line,
	}
}
