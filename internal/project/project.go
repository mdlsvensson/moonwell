// Package project reads a Moonwell project: it evaluates the manifest (moonwell.pkl, or moonwell.local.pkl when
// there is one) with pkl and turns the result into a typed Project.
package project

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	moonwell "github.com/mdlsvensson/moonwell"
	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/objects"
	"github.com/mdlsvensson/moonwell/internal/ordered"
	"github.com/mdlsvensson/moonwell/internal/proc"
	"github.com/mdlsvensson/moonwell/internal/settings"
	"github.com/mdlsvensson/moonwell/internal/text"
)

// Project is an evaluated manifest.
type Project struct {
	Root string
	// Manifest is the manifest that was evaluated, relative to Root: moonwell.local.pkl when it exists, else
	// moonwell.pkl.
	Manifest string
	Map      Map
	Build    Build
	Launch   Launch
	Yue      Yue
	Assets   Assets
	// Lint configures the unknown-global check.
	Lint Lint
	// Libraries holds the libraries of modules by key.
	Libraries ordered.Map[Library]
	Settings  *settings.Settings
	// Objects holds the custom objects, shape-checked only: resolving them needs the metadata and the source map.
	Objects objects.Manifest
}

// Map names the source map and the gameplay entry.
type Map struct{ Folder, Entry string }

// Build says where and how the map is packed.
type Build struct {
	Folder string
	Minify bool
}

// Launch says how the game is started.
type Launch struct {
	GameExecutable *string
	Args           []string
}

// Yue pins the YueScript compiler.
type Yue struct {
	Version string
	Path    *string
}

// Assets is the manifest's assets block.
type Assets struct {
	Paths   *ordered.Map[string]
	Exclude []string
}

// Lint is the manifest's lint block.
type Lint struct {
	UnknownGlobals string // "error" or "warning"
	Globals        []string
}

// Library is a library of YueScript and Lua modules: a GitHub tag, or a local folder when Path is set.
type Library struct {
	GitHub, Tag, Path *string
	// Dir is the folder inside the library that module names start from; empty for its root.
	Dir string
}

// EnsureLocalManifest creates moonwell.local.pkl in root unless it exists, and reports whether it did. It never
// overwrites.
func EnsureLocalManifest(root string) (bool, error) {
	file, err := os.OpenFile(filepath.Join(root, "moonwell.local.pkl"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o666)
	if errors.Is(err, fs.ErrExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if _, err := file.WriteString(LocalPkl()); err != nil {
		file.Close()
		return false, err
	}
	return true, file.Close()
}

// Load evaluates moonwell.local.pkl (or moonwell.pkl) in root with the pkl program (Pkl 0.32 or newer, as pkl.Ensure
// finds it) and returns the typed project.
func Load(ctx context.Context, root, pkl string, run proc.RunFunc) (*Project, error) {
	file := "moonwell.pkl"
	if fsx.Exists(filepath.Join(root, "moonwell.local.pkl")) {
		file = "moonwell.local.pkl"
	}
	if !fsx.Exists(filepath.Join(root, file)) {
		return nil, &diag.Error{
			Msg:  "No moonwell.pkl found in this directory.",
			File: root,
			Hint: "Run this command from a Moonwell project, or create one with `moonwell init <dir>`.",
		}
	}
	deps, err := os.ReadFile(filepath.Join(root, "PklProject.deps.json"))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, &diag.Error{
			Msg:  "PklProject.deps.json is missing.",
			File: "PklProject",
			Hint: "Run `pkl project resolve` in the project folder.",
		}
	}
	if err != nil {
		return nil, err
	}
	packageVersion, err := ReadPackageVersion(text.Lossy(deps))
	if err != nil {
		return nil, err
	}
	if err := CheckPackageVersion(packageVersion, moonwell.Version); err != nil {
		return nil, err
	}
	result, err := run(ctx, pkl, []string{"eval", "--format", "json", "--project-dir", ".", file}, proc.Options{Dir: root})
	if err != nil {
		return nil, err
	}
	if result.Code != 0 {
		output := result.Stderr
		if output == "" {
			output = result.Stdout
		}
		return nil, &diag.Error{Msg: "Evaluating " + file + " failed:\n" + text.Trim(output), File: file}
	}
	value, err := ordered.Decode([]byte(result.Stdout))
	if err != nil {
		return nil, &diag.Error{
			Msg:   "pkl eval printed output that is not valid JSON:\n" + text.TruncateUTF16(text.Trim(result.Stdout), 500),
			File:  file,
			Cause: err,
			Hint:  "Check that pkl on PATH is Pkl 0.32 or newer and that no other program is named pkl.",
		}
	}
	return Parse(root, value, file)
}

// parser keeps the first failure of a manifest check.
type parser struct {
	file string
	err  error
}

func (p *parser) fail(path, expected string) {
	if p.err == nil {
		p.err = &diag.Error{Msg: path + " must be " + expected + ".", File: p.file, Hint: objects.SchemaHint}
	}
}

var none = &ordered.Object{}

func (p *parser) record(input any, path string) *ordered.Object {
	if object, ok := input.(*ordered.Object); ok {
		return object
	}
	p.fail(path, "an object")
	return none
}

func (p *parser) string(object *ordered.Object, key, path string) string {
	value, _ := object.Get(key)
	if s, ok := value.(string); ok {
		return s
	}
	p.fail(path, "a string")
	return ""
}

func (p *parser) nullableString(object *ordered.Object, key, path string) *string {
	if value, _ := object.Get(key); value == nil {
		return nil
	}
	s := p.string(object, key, path)
	return &s
}

func (p *parser) boolean(object *ordered.Object, key, path string) bool {
	value, _ := object.Get(key)
	b, ok := value.(bool)
	if !ok {
		p.fail(path, "a boolean")
	}
	return b
}

func (p *parser) strings(object *ordered.Object, key, path string) []string {
	value, _ := object.Get(key)
	items, ok := value.([]any)
	out := []string{}
	for _, item := range items {
		s, isString := item.(string)
		ok = ok && isString
		out = append(out, s)
	}
	if !ok {
		p.fail(path, "a list of strings")
		return []string{}
	}
	return out
}

func (p *parser) stringRecord(object *ordered.Object, key, path string) *ordered.Map[string] {
	value, _ := object.Get(key)
	out := &ordered.Map[string]{}
	entries, ok := value.(*ordered.Object)
	if !ok {
		p.fail(path, "an object")
		return out
	}
	for name, entry := range entries.All() {
		s, isString := entry.(string)
		if !isString {
			p.fail(path, "a mapping of strings")
			return &ordered.Map[string]{}
		}
		out.Set(name, s)
	}
	return out
}

// Parse validates evaluated manifest JSON, a tree from ordered.Decode. Pkl omits null properties, so nullable
// fields may be absent.
func Parse(root string, value any, file string) (*Project, error) {
	p := &parser{file: file}
	data := p.record(value, "the manifest")
	field := func(key string) any {
		entry, _ := data.Get(key)
		return entry
	}
	mapBlock := p.record(field("map"), "map")
	build := p.record(field("build"), "build")
	launch := p.record(field("launch"), "launch")
	yue := p.record(field("yue"), "yue")
	// Moonwell 0.1.0 schema packages have no assets block; within 0.1.x a missing one means no configuration.
	var assets *ordered.Object
	if data.Has("assets") {
		assets = p.record(field("assets"), "assets")
	}
	// Every 0.4 schema package has a lint block; a manifest without one (unit fixtures) gets the defaults.
	var lint *ordered.Object
	if data.Has("lint") {
		lint = p.record(field("lint"), "lint")
	}
	project := &Project{Root: root, Manifest: file}
	// Every 0.5 schema package has a libraries block; a manifest without one (unit fixtures) has none.
	if data.Has("libraries") {
		for key, entry := range p.record(field("libraries"), "libraries").All() {
			path := "libraries[" + text.Quote(key) + "]"
			library := p.record(entry, path)
			parsed := Library{
				GitHub: p.nullableString(library, "github", path+".github"),
				Tag:    p.nullableString(library, "tag", path+".tag"),
				Path:   p.nullableString(library, "path", path+".path"),
			}
			if library.Has("dir") {
				parsed.Dir = p.string(library, "dir", path+".dir")
			}
			if p.err == nil && parsed.Path == nil && (parsed.GitHub == nil || parsed.Tag == nil) {
				p.err = &diag.Error{
					Msg:  path + " needs both github and tag, or a path.",
					File: file,
					Hint: `For example: ["example"] { github = "owner/repo"; tag = "v1.0.0" }, or path = "../my-library".`,
				}
			}
			project.Libraries.Set(key, parsed)
		}
	}
	if p.err != nil {
		return nil, p.err
	}
	var settingsTree any = none
	if data.Has("settings") {
		settingsTree = field("settings")
	}
	var err error
	if project.Settings, err = settings.Validate(settingsTree, file); err != nil {
		return nil, err
	}
	// Typed and raw gameplay constants that conflict need no map, so they fail here and name the evaluated
	// manifest.
	if _, err := settings.GameplaySections(project.Settings, file); err != nil {
		return nil, err
	}
	if project.Objects, err = objects.ParseManifest(field("objects"), data.Has("objects"), file); err != nil {
		return nil, err
	}

	project.Map = Map{Folder: p.string(mapBlock, "folder", "map.folder"), Entry: p.string(mapBlock, "entry", "map.entry")}
	project.Build = Build{
		Folder: p.string(build, "folder", "build.folder"),
		Minify: p.boolean(build, "minify", "build.minify"),
	}
	project.Launch = Launch{
		GameExecutable: p.nullableString(launch, "gameExecutable", "launch.gameExecutable"),
		Args:           p.strings(launch, "args", "launch.args"),
	}
	project.Yue = Yue{Version: p.string(yue, "version", "yue.version"), Path: p.nullableString(yue, "path", "yue.path")}
	project.Assets = Assets{Paths: &ordered.Map[string]{}, Exclude: []string{}}
	if assets != nil {
		project.Assets = Assets{
			Paths:   p.stringRecord(assets, "paths", "assets.paths"),
			Exclude: p.strings(assets, "exclude", "assets.exclude"),
		}
	}
	project.Lint = Lint{UnknownGlobals: "error", Globals: []string{}}
	if lint != nil {
		level, _ := lint.Get("unknownGlobals")
		if level != "error" && level != "warning" {
			p.fail("lint.unknownGlobals", `"error" or "warning"`)
		} else {
			project.Lint.UnknownGlobals = level.(string)
		}
		project.Lint.Globals = p.strings(lint, "globals", "lint.globals")
	}
	if p.err != nil {
		return nil, p.err
	}
	return project, nil
}

// LibraryKeys returns the keys of the manifest's libraries, in the manifest's order.
func (p *Project) LibraryKeys() []string {
	return p.Libraries.Keys()
}
