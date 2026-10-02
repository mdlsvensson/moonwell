package yue

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/mdlsvensson/moonwell/internal/bundle"
	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/layout"
	"github.com/mdlsvensson/moonwell/internal/ordered"
	"github.com/mdlsvensson/moonwell/internal/proc"
	"github.com/mdlsvensson/moonwell/internal/text"
)

// CompileOptions say what to compile and with which compiler.
type CompileOptions struct {
	// Yue is the path of the compiler.
	Yue  string
	Root string
	// Minify compiles with -m; without it, with -r, which keeps each statement on its source's line.
	Minify bool
	// Macros is where `import "moonwell.macros"` is found; nil gives the yue runs no --path.
	Macros *MacroSearch
	// Modules are the project's modules (bundle.CollectModules); nil lists the project's own from disk. Every
	// YueScript module among them is compiled.
	Modules []bundle.SourceModule
	// Run runs the compiler; nil is proc.Run.
	Run proc.RunFunc
	// Concurrency is how many compilers run at once; 0 is eight.
	Concurrency int
}

// Output is what a compile left in dist/stage/lua.
type Output struct {
	OutDir string
	// Hashes is the SHA-256 of each compiled source of src/, by its POSIX path under src/, such as
	// "heroes/captain.yue".
	Hashes map[string]string
	// Texts is the text of every compiled YueScript source, src/ and libraries, by its POSIX path from the project
	// root: the bytes that were hashed, as UTF-8.
	Texts map[string]string

	outputs map[string]string
	src     map[string]bundle.SourceModule
}

const (
	librariesPrefix = layout.LibrariesDir + "/"
	// hashesFormat marks a hashes file keyed by project path; an older one was keyed by the path under src/.
	hashesFormat = "paths|"
)

// outputPath is where a YueScript source compiles to, under dist/stage/lua: src/x.yue to x.lua, and
// .moonwell/libraries/<key>/x.yue to .libraries/<key>/x.lua. ok is false for a path in neither folder.
func outputPath(path string) (output string, ok bool) {
	lua := path
	if stem, isYue := strings.CutSuffix(path, ".yue"); isYue {
		lua = stem + ".lua"
	}
	if under, isSrc := strings.CutPrefix(lua, "src/"); isSrc {
		return under, true
	}
	if under, isLibrary := strings.CutPrefix(lua, librariesPrefix); isLibrary {
		return ".libraries/" + under, true
	}
	return "", false
}

// hashesFile is what dist/stage/lua/.hashes.json held: the settings of the compile and each source's hash.
type hashesFile struct {
	settings string
	files    map[string]string
}

func readHashes(path string) hashesFile {
	none := hashesFile{files: map[string]string{}}
	data, err := os.ReadFile(path)
	if err != nil {
		return none
	}
	tree, err := ordered.Decode(data)
	if err != nil {
		return none
	}
	document, ok := tree.(*ordered.Object)
	if !ok {
		return none
	}
	read := hashesFile{files: map[string]string{}}
	setting, _ := document.Get("settings")
	read.settings, _ = setting.(string)
	listed, _ := document.Get("files")
	if files, ok := listed.(*ordered.Object); ok {
		for name, value := range files.All() {
			if hash, isString := value.(string); isString {
				read.files[name] = hash
			}
		}
	}
	return read
}

// Compile compiles every YueScript module, of src/ and the libraries, into dist/stage/lua, recompiling only the
// files that changed since the last run.
func Compile(ctx context.Context, options CompileOptions) (*Output, error) {
	run := options.Run
	if run == nil {
		run = proc.Run
	}
	modules := options.Modules
	if modules == nil {
		var err error
		if modules, err = bundle.CollectModules(options.Root, bundle.ProjectRoots); err != nil {
			return nil, err
		}
	}
	output := &Output{
		OutDir:  filepath.Join(options.Root, "dist", "stage", "lua"),
		Hashes:  map[string]string{},
		Texts:   map[string]string{},
		outputs: map[string]string{},
		src:     map[string]bundle.SourceModule{},
	}
	outputOf := func(under string) string { return filepath.Join(output.OutDir, filepath.FromSlash(under)) }

	hashesPath := filepath.Join(output.OutDir, ".hashes.json")
	previous := readHashes(hashesPath)
	flavour, mode := "rewrite", "-r"
	if options.Minify {
		flavour, mode = "minify", "-m"
	}
	macrosHash := "no macros"
	if options.Macros != nil {
		macrosHash = options.Macros.Hash
	}
	settings := hashesFormat + options.Yue + "|" + flavour + "|" + macrosHash

	// Both are keyed by project path, like the hashes file.
	fileHashes := map[string]string{}
	var paths, pending []string
	for _, module := range modules {
		if module.Kind != bundle.Yue {
			continue
		}
		path := module.Path
		under, ok := outputPath(path)
		if !ok {
			return nil, errors.New("no compile output location for " + path)
		}
		data, err := os.ReadFile(filepath.Join(options.Root, filepath.FromSlash(path)))
		if err != nil {
			return nil, err
		}
		paths = append(paths, path)
		output.outputs[path] = outputOf(under)
		fileHashes[path] = fsx.SHA256Hex(data)
		output.Texts[path] = text.Decode(data)
		upToDate := previous.settings == settings && previous.files[path] == fileHashes[path] && fsx.Exists(output.outputs[path])
		if !upToDate {
			pending = append(pending, path)
		}
		if strings.HasPrefix(path, "src/") {
			output.src[module.Name] = module
		}
	}
	// The output of a source that is gone goes too. An older file's keys name no output, so its outputs stay.
	if strings.HasPrefix(previous.settings, hashesFormat) {
		for path := range previous.files {
			if _, current := fileHashes[path]; current {
				continue
			}
			if under, ok := outputPath(path); ok {
				if err := fsx.RemoveFile(outputOf(under)); err != nil {
					return nil, err
				}
			}
		}
	}

	var lock sync.Mutex
	var failures []*diag.Error
	err := forEachLimited(pending, options.Concurrency, func(path string) error {
		target := output.outputs[path]
		if err := os.MkdirAll(filepath.Dir(target), 0o777); err != nil {
			return err
		}
		args := []string{"--target=5.3", mode, "-o", target}
		args = append(args, options.Macros.PathArgs()...)
		args = append(args, filepath.Join(options.Root, filepath.FromSlash(path)))
		result, err := run(ctx, options.Yue, args, proc.Options{})
		if err != nil {
			return err
		}
		lock.Lock()
		source := output.Texts[path]
		lock.Unlock()
		var failure *diag.Error
		if result.Code != 0 {
			failure = CompileError(path, result.Stdout+"\n"+result.Stderr)
		} else {
			failure = emptyOutputError(path, target, source)
		}
		if failure == nil {
			return nil
		}
		if err := fsx.RemoveFile(target); err != nil {
			return err
		}
		lock.Lock()
		defer lock.Unlock()
		delete(fileHashes, path)
		delete(output.Texts, path)
		failures = append(failures, failure)
		return nil
	})
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(output.OutDir, 0o777); err != nil {
		return nil, err
	}
	files := &ordered.Map[any]{}
	for _, path := range paths {
		if hash, compiled := fileHashes[path]; compiled {
			files.Set(path, hash)
		}
	}
	document := &ordered.Map[any]{}
	document.Set("settings", settings)
	document.Set("files", files)
	if err := os.WriteFile(hashesPath, []byte(ordered.Stringify(document, 2)), 0o666); err != nil {
		return nil, err
	}
	if len(failures) > 0 {
		first := firstFailure(failures)
		if len(failures) == 1 {
			return nil, first
		}
		return nil, &diag.Error{
			Msg:  first.Msg + "\n(" + strconv.Itoa(len(failures)-1) + " more file(s) failed to compile)",
			File: first.File,
			Line: first.Line,
		}
	}

	for path, hash := range fileHashes {
		if under, isSrc := strings.CutPrefix(path, "src/"); isSrc {
			output.Hashes[under] = hash
		}
	}
	return output, nil
}

// firstFailure is the failure of the file that sorts first, as JavaScript's localeCompare sorts.
func firstFailure(failures []*diag.Error) *diag.Error {
	slices.SortStableFunc(failures, func(a, b *diag.Error) int { return text.LocaleCompare(a.File, b.File) })
	return failures[0]
}

// Load gives a src/ module's compiled output by its dotted name; nil when there is none.
func (o *Output) Load(name string) (*bundle.CompiledModule, error) {
	module, found := o.src[name]
	if !found {
		return nil, nil
	}
	return o.LoadModule(module)
}

// LoadModule gives any compiled YueScript module's output; nil when it has none. Its SourcePath is the module's
// path from the project root.
func (o *Output) LoadModule(module bundle.SourceModule) (*bundle.CompiledModule, error) {
	if _, compiled := o.Texts[module.Path]; module.Kind != bundle.Yue || !compiled {
		return nil, nil
	}
	data, err := os.ReadFile(o.outputs[module.Path])
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &bundle.CompiledModule{Name: module.Name, SourcePath: module.Path, Source: text.Lossy(data)}, nil
}

var blankOrComment = regexp.MustCompile(`^` + text.SpaceClass + `*(--` + text.NotLineBreak + `*)?$`)

// emptyOutputError is the failure of a compile that reported success and wrote an empty file: yue 0.34.2 does that
// for a source that uses floor division (`//`), with both -r and -m, and the module would silently vanish from the
// build. A source without code (blank and comment lines only) is rightly empty.
func emptyOutputError(file, output, source string) *diag.Error {
	info, err := os.Stat(output)
	if err != nil || info.Size() != 0 {
		return nil
	}
	hasCode := false
	for _, line := range strings.Split(strings.ReplaceAll(source, "\r\n", "\n"), "\n") {
		if !blankOrComment.MatchString(line) {
			hasCode = true
			break
		}
	}
	if !hasCode {
		return nil
	}
	return &diag.Error{
		Msg:  "YueScript reported success but wrote no Lua for " + file + ", although the file has code.",
		File: file,
		Hint: "YueScript 0.34.2 does this for a file that uses the floor division operator `//`. " +
			"Write math.floor(a / b) instead.",
	}
}

var (
	lineBreak     = regexp.MustCompile(`\r?\n`)
	numberedLine  = regexp.MustCompile(`(?:\A|[\n\r\x{2028}\x{2029}])([0-9]+): (` + text.NotLineBreak + `+)`)
	macroPosition = regexp.MustCompile(`^failed to expand macro: \(macro [^)]*\):[0-9]+: `)
)

// CompileError turns the compiler's output for a failed file into an error at the file's line. yue prints "Failed to
// compile: <file>", then "<line>: <message>" and a source excerpt. A failing macro's message starts with "failed to
// expand macro: (macro <name>):<line>: ", a line of the macro module rather than of the file, so the first line of
// the error drops it; the excerpt keeps the compiler's full text.
func CompileError(file, output string) *diag.Error {
	var kept []string
	for _, line := range lineBreak.Split(output, -1) {
		if !strings.HasPrefix(line, "Failed to compile") {
			kept = append(kept, line)
		}
	}
	detail := text.Trim(strings.Join(kept, "\n"))
	match := numberedLine.FindStringSubmatch(output)
	if match == nil {
		if detail == "" {
			detail = "YueScript compilation failed."
		}
		return &diag.Error{Msg: detail, File: file}
	}
	line, _ := strconv.Atoi(match[1])
	return &diag.Error{Msg: macroPosition.ReplaceAllString(match[2], "") + "\n" + detail, File: file, Line: line}
}

// forEachLimited runs fn for every item, at most limit at a time (eight for 0), and returns the first error. After
// an error no further item is started.
func forEachLimited(items []string, limit int, fn func(item string) error) error {
	if limit <= 0 {
		limit = 8
	}
	var group sync.WaitGroup
	var lock sync.Mutex
	var first error
	slots := make(chan struct{}, limit)
	for _, item := range items {
		slots <- struct{}{}
		lock.Lock()
		failed := first != nil
		lock.Unlock()
		if failed {
			<-slots
			break
		}
		group.Add(1)
		go func() {
			defer group.Done()
			defer func() { <-slots }()
			if err := fn(item); err != nil {
				lock.Lock()
				defer lock.Unlock()
				if first == nil {
					first = err
				}
			}
		}()
	}
	group.Wait()
	return first
}
