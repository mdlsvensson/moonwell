// Package moonwell holds the files the moonwell executable carries: the project template, the Lua runtime that goes
// into maps, and the game data.
package moonwell

import (
	"embed"
	"io/fs"
	"slices"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/text"
)

//go:embed all:template
var templateFS embed.FS

// RuntimeLua is the Lua runtime bundled into every map.
//
//go:embed cli/runtime/moonwell.lua
var RuntimeLua string

// MacrosYue is the macro module projects import as "moonwell.macros".
//
//go:embed cli/runtime/macros.yue
var MacrosYue string

// GamePaths is the list of paths the game ships, one per line.
//
//go:embed cli/data/game-paths.txt
var GamePaths string

// Metadata is the object-data metadata, as JSON.
//
//go:embed cli/data/metadata.json
var Metadata []byte

// Natives is the game's natives, as JSON.
//
//go:embed cli/data/natives.json
var Natives []byte

// TemplateFile is one file of the project template.
type TemplateFile struct {
	Path string // relative to the project, with "/"
	Data []byte
}

// TemplateExclude names the template files init writes itself, or never writes.
var TemplateExclude = []string{"deno.json", "PklProject", "PklProject.deps.json", "moonwell.local.pkl"}

// TemplateFiles returns every file init copies from the template, sorted by path.
func TemplateFiles() ([]TemplateFile, error) {
	var files []TemplateFile
	err := fs.WalkDir(templateFS, "template", func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		rel := strings.TrimPrefix(path, "template/")
		if slices.Contains(TemplateExclude, rel) || isGenerated(rel) {
			return nil
		}
		data, err := templateFS.ReadFile(path)
		if err != nil {
			return err
		}
		files = append(files, TemplateFile{Path: rel, Data: data})
		return nil
	})
	slices.SortFunc(files, func(a, b TemplateFile) int { return text.Compare(a.Path, b.Path) })
	return files, err
}

// isGenerated reports what builds, check, setup and the YueScript extension write inside a project. Those files are
// git-ignored there and never part of the template, even when the commands have run in template/.
func isGenerated(path string) bool {
	return strings.HasPrefix(path, "dist/") || strings.HasPrefix(path, ".moonwell/") ||
		(strings.HasPrefix(path, "src/") && strings.HasSuffix(path, ".lua"))
}
