package moonwell

import (
	"embed"
	"io/fs"
	"slices"
	"strings"
)

//go:embed all:template
var templateFS embed.FS

//go:embed runtime/moonwell.lua
var RuntimeLua string

//go:embed runtime/macros.yue
var MacrosYue string

//go:embed data/game-paths.txt
var GamePaths string

//go:embed data/metadata.json
var Metadata []byte

//go:embed data/natives.json
var Natives []byte

type TemplateFile struct {
	Path string
	Data []byte
}

var TemplateExclude = []string{"PklProject", "PklProject.deps.json", "moonwell.local.pkl"}

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
	slices.SortFunc(files, func(a, b TemplateFile) int { return strings.Compare(a.Path, b.Path) })
	return files, err
}

func isGenerated(path string) bool {
	return strings.HasPrefix(path, "dist/") || strings.HasPrefix(path, ".moonwell/") ||
		(strings.HasPrefix(path, "src/") && strings.HasSuffix(path, ".lua"))
}
