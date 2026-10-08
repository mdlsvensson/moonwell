package editor

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	moonwell "github.com/mdlsvensson/moonwell"
	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/manifest"
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
	there, err := lookAtFiles(root)
	if err != nil {
		return nil, err
	}
	added, err := addMissingFiles(root, template, there)
	if err != nil {
		return nil, err
	}
	ignores, err := addIgnores(root)
	if err != nil {
		return nil, err
	}
	if len(ignores) > 0 {
		added = append(added, gitignoreFile+" ("+strings.Join(ignores, ", ")+")")
	}
	return added, nil
}

func lookAtFiles(root string) (there map[string]bool, err error) {
	there = map[string]bool{}
	for _, file := range append(slices.Clone(editorFiles), gitignoreFile) {
		if there[file], err = isThere(root, file); err != nil {
			return nil, err
		}
	}
	return there, nil
}

func isThere(root, file string) (bool, error) {
	at := onDisk(root, file)
	info, err := fsx.Lstat(at)
	switch {
	case err != nil || info == nil:
		return false, nil
	case fsx.IsSymlink(info) && leadsNowhere(at):
		return false, errLinkToNothing(file)
	}
	return true, nil
}

func leadsNowhere(link string) bool {
	_, err := os.Stat(link)
	return errors.Is(err, fs.ErrNotExist)
}

func addMissingFiles(root string, template []moonwell.TemplateFile, there map[string]bool) ([]string, error) {
	added := []string{}
	for _, file := range editorFiles {
		if there[file] {
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

func addIgnores(root string) ([]string, error) {
	held, _, err := readIfThere(root, gitignoreFile)
	if err != nil {
		return nil, err
	}
	lacking := ignoresLacking(held)
	if len(lacking) == 0 {
		return nil, nil
	}
	if err := writeFile(root, gitignoreFile, withLines(held, lacking)); err != nil {
		return nil, err
	}
	return lacking, nil
}

func ignoresLacking(held []byte) []string {
	var lines []string
	for line := range strings.SplitSeq(fsx.TrimBOM(string(held)), "\n") {
		lines = append(lines, strings.Trim(line, lineSpace))
	}
	var lacking []string
	for _, ignore := range gitignoreLines {
		if !slices.Contains(lines, ignore) {
			lacking = append(lacking, ignore)
		}
	}
	return lacking
}

func withLines(held []byte, lines []string) []byte {
	out := bytes.Clone(held)
	if len(out) > 0 && !bytes.HasSuffix(out, []byte("\n")) {
		out = append(out, '\n')
	}
	for _, line := range lines {
		out = append(append(out, line...), '\n')
	}
	return out
}

var luarcArrays = []string{"runtime.path", "workspace.library", "workspace.ignoreDir"}

func MergeLuarc(root string, template []moonwell.TemplateFile) (added []string, merged bool, err error) {
	written, found, err := readIfThere(root, luarcFile)
	switch {
	case err != nil:
		return nil, false, err
	case !found:
		return []string{}, true, nil
	}
	entries, err := LuarcTemplateEntries(template)
	if err != nil {
		return nil, false, err
	}
	config, isObject := objectOf(written)
	if !isObject {
		return nil, false, nil
	}
	added = addLacking(&config, entries)
	if len(added) == 0 {
		return added, true, nil
	}
	if err := writeLuarc(root, config); err != nil {
		return nil, false, err
	}
	return added, true, nil
}

func addLacking(config *manifest.OrderedMap[json.RawMessage], entries map[string][]string) []string {
	added := []string{}
	for _, key := range luarcArrays {
		added = append(added, addEntries(config, key, entries[key])...)
	}
	return added
}

func LuarcTemplateEntries(template []moonwell.TemplateFile) (map[string][]string, error) {
	data, err := templateFile(template, luarcFile)
	if err != nil {
		return nil, err
	}
	config, isObject := objectOf(data)
	if !isObject {
		return nil, errors.New("editor: the template's .luarc.json is not a JSON object")
	}
	entries := map[string][]string{}
	for _, key := range luarcArrays {
		written, _ := config.Get(key)
		list, isList := stringsOf(written)
		if !isList {
			return nil, errors.New("editor: the template's .luarc.json has no " + key + " array of strings")
		}
		entries[key] = list
	}
	return entries, nil
}

func objectOf(text []byte) (config manifest.OrderedMap[json.RawMessage], isObject bool) {
	text = fsx.TrimBOM(text)
	if !startsWith(text, '{') || json.Unmarshal(text, &config) != nil {
		return manifest.OrderedMap[json.RawMessage]{}, false
	}
	return config, true
}

func addEntries(config *manifest.OrderedMap[json.RawMessage], key string, entries []string) (added []string) {
	written, given := config.Get(key)
	elements, isArray := elementsOf(written)
	if given && !isArray {
		return nil
	}
	held := map[string]bool{}
	for _, element := range elements {
		if text, isString := stringOf(element); isString {
			held[text] = true
		}
	}
	for _, entry := range entries {
		if held[entry] {
			continue
		}
		held[entry] = true
		elements = append(elements, json.RawMessage(fsx.QuoteJSON(entry)))
		added = append(added, entry)
	}
	config.Set(key, arrayOf(elements))
	return added
}

func elementsOf(written json.RawMessage) (elements []json.RawMessage, isArray bool) {
	if !startsWith(written, '[') || json.Unmarshal(written, &elements) != nil {
		return nil, false
	}
	return elements, true
}

func stringOf(written json.RawMessage) (text string, isString bool) {
	if !startsWith(written, '"') || json.Unmarshal(written, &text) != nil {
		return "", false
	}
	return text, true
}

func startsWith(text []byte, first byte) bool {
	text = bytes.TrimLeft(text, jsonSpace)
	return len(text) > 0 && text[0] == first
}

func stringsOf(written json.RawMessage) (list []string, isList bool) {
	elements, isArray := elementsOf(written)
	if !isArray {
		return nil, false
	}
	for _, element := range elements {
		text, isString := stringOf(element)
		if !isString {
			return nil, false
		}
		list = append(list, text)
	}
	return list, true
}

func arrayOf(elements []json.RawMessage) json.RawMessage {
	array := json.RawMessage("[")
	for i, element := range elements {
		if i > 0 {
			array = append(array, ',')
		}
		array = append(array, element...)
	}
	return append(array, ']')
}

func writeLuarc(root string, config manifest.OrderedMap[json.RawMessage]) error {
	text, err := laidOut(config)
	if err != nil {
		return err
	}
	return writeFile(root, luarcFile, text)
}

func laidOut(config manifest.OrderedMap[json.RawMessage]) ([]byte, error) {
	var onOneLine bytes.Buffer
	onOneLine.WriteByte('{')
	for key, value := range config.All() {
		if onOneLine.Len() > 1 {
			onOneLine.WriteByte(',')
		}
		onOneLine.WriteString(fsx.QuoteJSON(key))
		onOneLine.WriteByte(':')
		onOneLine.Write(value)
	}
	onOneLine.WriteByte('}')
	var text bytes.Buffer
	if err := json.Indent(&text, onOneLine.Bytes(), "", "  "); err != nil {
		return nil, err
	}
	text.WriteByte('\n')
	return text.Bytes(), nil
}

func onDisk(root, file string) string {
	return filepath.Join(root, filepath.FromSlash(file))
}

func readIfThere(root, file string) (data []byte, found bool, err error) {
	if data, found, err = fsx.ReadFileIfExists(onDisk(root, file)); err != nil {
		return nil, false, errNotRead(file, err)
	}
	return data, found, nil
}

func writeFile(root, file string, data []byte) error {
	target := onDisk(root, file)
	err := os.MkdirAll(filepath.Dir(target), 0o777)
	if err == nil {
		err = os.WriteFile(target, data, 0o666)
	}
	if err != nil {
		return errNotWritten(file, err)
	}
	return nil
}

func errLinkToNothing(file string) error {
	return &diag.Error{
		Msg:  file + " is a link to a file that does not exist.",
		File: file,
		Hint: "Remove the link, or create the file it leads to, then run setup again.",
	}
}

func errNotRead(file string, cause error) error {
	return errFile("Reading", file, cause)
}

func errNotWritten(file string, cause error) error {
	return errFile("Writing", file, cause)
}

func errFile(action, file string, cause error) error {
	return &diag.Error{
		Msg:  action + " " + file + " failed: " + fsx.Reason(cause),
		File: file,
		Hint: "Close any program that has " + file + " open and check that it is a file you can read and write, then run " +
			"setup again.",
		Cause: cause,
	}
}
