package editor

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	moonwell "github.com/mdlsvensson/moonwell"
	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/fsx"
)

var editorFiles = []string{"yueconfig.yue", ".luarc.json", ".vscode/extensions.json"}

var gitignoreLines = []string{".moonwell/", "src/**/*.lua"}

const (
	gitignoreFile = ".gitignore"
	luarcFile     = ".luarc.json"
)

const (
	jsonSpace = " \t\r\n"
	lineSpace = " \t\r\v\f"
)

func AddFiles(root string, template []moonwell.TemplateFile) ([]string, error) {
	existing, err := findExistingFiles(root)
	if err != nil {
		return nil, err
	}
	added, err := addMissingFiles(root, template, existing)
	if err != nil {
		return nil, err
	}
	ignoreLines, err := addGitignoreLines(root)
	if err != nil {
		return nil, err
	}
	if len(ignoreLines) > 0 {
		added = append(added, gitignoreFile+" ("+strings.Join(ignoreLines, ", ")+")")
	}
	return added, nil
}

func findExistingFiles(root string) (existing map[string]bool, err error) {
	existing = map[string]bool{}
	for _, file := range append(slices.Clone(editorFiles), gitignoreFile) {
		if existing[file], err = fileExists(root, file); err != nil {
			return nil, err
		}
	}
	return existing, nil
}

func fileExists(root, path string) (bool, error) {
	fullPath := fullPathOf(root, path)
	info, err := fsx.Lstat(fullPath)
	switch {
	case err != nil || info == nil:
		return false, nil
	case fsx.IsSymlink(info) && isBrokenSymlink(fullPath):
		return false, errBrokenSymlink(path)
	}
	return true, nil
}

func isBrokenSymlink(symlink string) bool {
	_, err := os.Stat(symlink)
	return errors.Is(err, fs.ErrNotExist)
}

func addMissingFiles(root string, template []moonwell.TemplateFile, existing map[string]bool) ([]string, error) {
	added := []string{}
	for _, file := range editorFiles {
		if existing[file] {
			continue
		}
		data, err := templateFile(template, file)
		if err != nil {
			return nil, err
		}
		if err := writeFile(root, file, data); err != nil {
			return nil, err
		}
		added = append(added, file)
	}
	return added, nil
}

func templateFile(template []moonwell.TemplateFile, path string) ([]byte, error) {
	for _, file := range template {
		if file.Path == path {
			return file.Data, nil
		}
	}
	return nil, errors.New("editor: the template has no " + path + "; pass moonwell.TemplateFiles()")
}

func addGitignoreLines(root string) ([]string, error) {
	content, _, err := readFileIfExists(root, gitignoreFile)
	if err != nil {
		return nil, err
	}
	missing := missingGitignoreLines(content)
	if len(missing) == 0 {
		return nil, nil
	}
	if err := writeFile(root, gitignoreFile, appendLines(content, missing)); err != nil {
		return nil, err
	}
	return missing, nil
}

func missingGitignoreLines(content []byte) []string {
	var lines []string
	for line := range strings.SplitSeq(fsx.TrimBOM(string(content)), "\n") {
		lines = append(lines, strings.Trim(line, lineSpace))
	}
	var missing []string
	for _, ignoreLine := range gitignoreLines {
		if !slices.Contains(lines, ignoreLine) {
			missing = append(missing, ignoreLine)
		}
	}
	return missing
}

func appendLines(content []byte, lines []string) []byte {
	out := bytes.Clone(content)
	if len(out) > 0 && !bytes.HasSuffix(out, []byte("\n")) {
		out = append(out, '\n')
	}
	for _, line := range lines {
		out = append(append(out, line...), '\n')
	}
	return out
}

func fullPathOf(root, file string) string {
	return filepath.Join(root, filepath.FromSlash(file))
}

func readFileIfExists(root, path string) (data []byte, found bool, err error) {
	if data, found, err = fsx.ReadFileIfExists(fullPathOf(root, path)); err != nil {
		return nil, false, errNotRead(path, err)
	}
	return data, found, nil
}

func writeFile(root, path string, data []byte) error {
	fullPath := fullPathOf(root, path)
	err := os.MkdirAll(filepath.Dir(fullPath), 0o777)
	if err == nil {
		err = os.WriteFile(fullPath, data, 0o666)
	}
	if err != nil {
		return errNotWritten(path, err)
	}
	return nil
}

func errBrokenSymlink(path string) error {
	return &diag.Error{
		Msg:  path + " is a link to a file that does not exist.",
		File: path,
		Hint: "Remove the link, or create the file it leads to, then run setup again.",
	}
}

func errNotRead(path string, cause error) error {
	return errFile("Reading", path, cause)
}

func errNotWritten(path string, cause error) error {
	return errFile("Writing", path, cause)
}

func errFile(action, path string, cause error) error {
	return &diag.Error{
		Msg:  action + " " + path + " failed: " + fsx.Reason(cause),
		File: path,
		Hint: "Close any program that has " + path + " open and check that it is a file you can read and write, then run " +
			"setup again.",
		Cause: cause,
	}
}
