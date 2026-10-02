package editor

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	moonwell "github.com/mdlsvensson/moonwell"
	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/ordered"
	"github.com/mdlsvensson/moonwell/internal/text"
)

// Files are the committed files VS Code's YueScript extension and lua-language-server read.
var Files = []string{"yueconfig.yue", ".luarc.json", ".vscode/extensions.json"}

// Ignores are the .gitignore lines for what Moonwell and the extension write.
var Ignores = []string{".moonwell/", "src/**/*.lua"}

var lineBreak = regexp.MustCompile(`\r?\n`)

// templateFile finds a file of the template; nil stands for the embedded template.
func templateFile(template []moonwell.TemplateFile, path string) ([]byte, error) {
	if template == nil {
		var err error
		if template, err = moonwell.TemplateFiles(); err != nil {
			return nil, err
		}
	}
	for _, file := range template {
		if file.Path == path {
			return file.Data, nil
		}
	}
	return nil, errors.New("the embedded template has no " + path)
}

// AddFiles gives a project created before the editor files the ones it lacks: each missing file from the template,
// and each missing .gitignore line appended. It never overwrites a file. It returns what it added. A nil template
// is the embedded one.
func AddFiles(root string, template []moonwell.TemplateFile) ([]string, error) {
	added := []string{}
	for _, path := range Files {
		target := filepath.Join(root, filepath.FromSlash(path))
		if fsx.Exists(target) {
			continue
		}
		data, err := templateFile(template, path)
		if err != nil {
			return nil, err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o777); err != nil {
			return nil, err
		}
		if err := os.WriteFile(target, data, 0o666); err != nil {
			return nil, err
		}
		added = append(added, path)
	}
	gitignore := filepath.Join(root, ".gitignore")
	current := ""
	if fsx.Exists(gitignore) {
		data, err := os.ReadFile(gitignore)
		if err != nil {
			return nil, err
		}
		current = text.Lossy(data)
	}
	var lines []string
	for _, line := range lineBreak.Split(current, -1) {
		lines = append(lines, text.Trim(line))
	}
	var missing []string
	for _, line := range Ignores {
		if !slices.Contains(lines, line) {
			missing = append(missing, line)
		}
	}
	if len(missing) > 0 {
		separator := "\n"
		if current == "" || strings.HasSuffix(current, "\n") {
			separator = ""
		}
		if err := os.WriteFile(gitignore, []byte(current+separator+strings.Join(missing, "\n")+"\n"), 0o666); err != nil {
			return nil, err
		}
		added = append(added, ".gitignore ("+strings.Join(missing, ", ")+")")
	}
	return added, nil
}

// LuarcArrays are the .luarc.json arrays setup keeps up to date in a project made by an older Moonwell.
var LuarcArrays = []string{"runtime.path", "workspace.library", "workspace.ignoreDir"}

// LuarcTemplateEntries are the template's runtime.path, workspace.library and workspace.ignoreDir entries, from its
// .luarc.json. A nil template is the embedded one.
func LuarcTemplateEntries(template []moonwell.TemplateFile) (map[string][]string, error) {
	data, err := templateFile(template, ".luarc.json")
	if err != nil {
		return nil, err
	}
	tree, err := ordered.Decode([]byte(text.Decode(data)))
	if err != nil {
		return nil, err
	}
	config, ok := tree.(*ordered.Object)
	if !ok {
		return nil, errors.New("the embedded template's .luarc.json is not a JSON object")
	}
	entries := map[string][]string{}
	for _, key := range LuarcArrays {
		value, _ := config.Get(key)
		list, isList := value.([]any)
		if !isList {
			return nil, errors.New("the embedded template's .luarc.json has no " + key + " array")
		}
		for _, entry := range list {
			name, isString := entry.(string)
			if !isString {
				name = ordered.Stringify(entry, 0)
			}
			entries[key] = append(entries[key], name)
		}
	}
	return entries, nil
}

// MergeLuarc adds the template's runtime.path, workspace.library and workspace.ignoreDir entries that the project's
// .luarc.json lacks, keeping every other key and value, and rewrites the file as formatted JSON when it adds any. It
// returns the entries it added. merged is false when the file is not a JSON object, which is then left alone. A
// missing file adds nothing; a leading byte order mark is ignored.
func MergeLuarc(root string, template []moonwell.TemplateFile) (added []string, merged bool, err error) {
	path := filepath.Join(root, ".luarc.json")
	added = []string{}
	if !fsx.Exists(path) {
		return added, true, nil
	}
	entries, err := LuarcTemplateEntries(template)
	if err != nil {
		return nil, false, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false, luarcError("Reading", err)
	}
	tree, err := ordered.Decode([]byte(strings.TrimPrefix(text.Lossy(data), "\xEF\xBB\xBF")))
	if err != nil {
		return nil, false, nil
	}
	config, ok := tree.(*ordered.Object)
	if !ok {
		return nil, false, nil
	}
	for _, key := range LuarcArrays {
		current, given := config.Get(key)
		if !given {
			list := make([]any, len(entries[key]))
			for i, entry := range entries[key] {
				list[i] = entry
			}
			config.Set(key, list)
			added = append(added, entries[key]...)
			continue
		}
		list, isList := current.([]any)
		if !isList {
			continue
		}
		for _, entry := range entries[key] {
			if !slices.Contains(list, any(entry)) {
				list = append(list, entry)
				added = append(added, entry)
			}
		}
		config.Set(key, list)
	}
	if len(added) > 0 {
		if err := os.WriteFile(path, []byte(ordered.Stringify(config, 2)+"\n"), 0o666); err != nil {
			return nil, false, luarcError("Writing", err)
		}
	}
	return added, true, nil
}

func luarcError(action string, cause error) *diag.Error {
	return &diag.Error{
		Msg:   action + " .luarc.json failed: " + fsx.Reason(cause),
		File:  ".luarc.json",
		Cause: cause,
		Hint: "Close any program that has .luarc.json open and check that it is a file you can read and write, then run " +
			"setup again.",
	}
}
