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

// files are the committed files VS Code's YueScript extension and lua-language-server read: the editor files.
var files = []string{"yueconfig.yue", ".luarc.json", ".vscode/extensions.json"}

// ignores are the .gitignore lines for what Moonwell and the extension write.
var ignores = []string{".moonwell/", "src/**/*.lua"}

// The two files of a project that are changed where they are, from the project folder.
const (
	gitignoreFile = ".gitignore"
	luarcFile     = ".luarc.json"
)

const (
	jsonSpace = " \t\r\n"   // the white space of JSON
	lineSpace = " \t\r\v\f" // the white space of ASCII that a line may hold
)

// AddFiles gives a project the editor files it lacks: each missing file from the template, and each missing
// .gitignore line appended. It never overwrites a file. It returns what it added.
//
// What is added is named as setup reports it: each file by its path, in the order of files, and then .gitignore
// with the lines it was given.
//
// The files are the project's own, which its user commits, and so is a link at one of them. A file behind a link
// is there: one of files is left as it is, and .gitignore is read and written through the link. A link that leads
// to nothing, at one of files or at .gitignore, is refused before anything is written, since a file written under
// its name would be made where the link leads. A link to a folder at .vscode is followed.
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

// lookAtFiles reports, for each of files and for .gitignore, whether there is something under its name. All four
// are looked at before any is written, so that a link to nothing at one of them leaves the project as it is.
func lookAtFiles(root string) (there map[string]bool, err error) {
	there = map[string]bool{}
	for _, file := range append(slices.Clone(files), gitignoreFile) {
		if there[file], err = isThere(root, file); err != nil {
			return nil, err
		}
	}
	return there, nil
}

// isThere reports whether there is something under the name of a file of the project: a file, a folder, or a link
// that leads somewhere. The name itself is looked at, and no link is followed to answer; a link that leads to
// nothing is refused. A look that fails is taken for nothing there: the write that follows says what is in the
// way.
func isThere(root, file string) (bool, error) {
	at := onDisk(root, file)
	info, err := fsx.Lstat(at)
	switch {
	case err != nil || info == nil:
		return false, nil
	case fsx.IsLink(info) && leadsNowhere(at):
		return false, errLinkToNothing(file)
	}
	return true, nil
}

// leadsNowhere reports whether there is nothing where the link at a path leads.
func leadsNowhere(link string) bool {
	_, err := os.Stat(link)
	return errors.Is(err, fs.ErrNotExist)
}

// addMissingFiles writes each of files that is not there, from the template, and returns those it wrote. What is
// there under the name of a file stays as it is, whatever it holds, and a folder too.
func addMissingFiles(root string, template []moonwell.TemplateFile, there map[string]bool) ([]string, error) {
	added := []string{}
	for _, file := range files {
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

// templateFile is the bytes of the template's file at path.
func templateFile(template []moonwell.TemplateFile, path string) ([]byte, error) {
	for _, file := range template {
		if file.Path == path {
			return file.Data, nil
		}
	}
	// A plain error: the template is Moonwell's own and holds each of files, so one without the file is a mistake
	// in Moonwell and nothing the user can put right.
	return nil, errors.New("editor: the template has no " + path + "; pass moonwell.TemplateFiles()")
}

// addIgnores appends each of ignores that .gitignore lacks, and returns those it appended. A project without the
// file is given one.
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

// ignoresLacking is each of ignores that no line of a .gitignore is. A line ends at a line feed, and is compared
// without the white space of ASCII around it; a byte order mark at the start of the file is no part of the first.
func ignoresLacking(held []byte) []string {
	var lines []string
	for line := range strings.SplitSeq(fsx.WithoutMark(string(held)), "\n") {
		lines = append(lines, strings.Trim(line, lineSpace))
	}
	var lacking []string
	for _, ignore := range ignores {
		if !slices.Contains(lines, ignore) {
			lacking = append(lacking, ignore)
		}
	}
	return lacking
}

// withLines is a .gitignore with lines appended, each ended by a line feed. Every byte the file holds stays as it
// is, and a last line without a line break is given one first.
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

// luarcArrays are the .luarc.json arrays setup keeps up to date.
var luarcArrays = []string{"runtime.path", "workspace.library", "workspace.ignoreDir"}

// MergeLuarc adds the template's entries of those arrays that the project's .luarc.json lacks, keeps every other
// key and value as written, and rewrites the file when it adds any. merged is false when the file is not a JSON
// object, which is then left alone. A missing file adds nothing.
//
// An entry is there when the array holds the same string, however it is written; an array that is not there is
// given whole, at the end of the file, and a value under its key that is no array stays. Where the file is
// rewritten, it is laid out with two spaces and a final line break, without a byte order mark: a key keeps its
// place, and a value the text of each of its tokens, a number and a string as they are written. A key of the file
// itself that comes twice is then written once, at its first place and with its last value, which is the value
// that was merged.
//
// A file behind a link is read and written through the link. A link that leads to nothing is no file: nothing is
// added, and nothing is written.
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

// addLacking gives each of luarcArrays the entries it lacks, and returns all that it added, array after array.
func addLacking(config *manifest.Ordered[json.RawMessage], entries map[string][]string) []string {
	added := []string{}
	for _, key := range luarcArrays {
		added = append(added, addEntries(config, key, entries[key])...)
	}
	return added
}

// LuarcTemplateEntries is the entries of each of luarcArrays in the template's .luarc.json, under the key of the
// array and in the order the template lists them: what MergeLuarc adds where a project lacks it, and what setup
// names to the user of a .luarc.json that MergeLuarc left alone. An array without entries has a nil list.
//
// Its failures are plain errors: the template is Moonwell's own, and its .luarc.json is an object with the three
// arrays of strings, so one that is not is a mistake in Moonwell and nothing the user can put right.
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

// objectOf reads the text of a .luarc.json as the members of a JSON object: each key, and its value as it is
// written, in the order of the text. A key that comes twice keeps its first place and its last value. A byte
// order mark before the text is read past. It reports false for text that is not JSON, which has neither the
// comments nor the comma after a last member that lua-language-server reads, and for JSON that is no object.
func objectOf(text []byte) (config manifest.Ordered[json.RawMessage], isObject bool) {
	text = fsx.WithoutMark(text)
	// A null reads as a mapping without keys, so what the text starts with is asked first.
	if !startsWith(text, '{') || json.Unmarshal(text, &config) != nil {
		return manifest.Ordered[json.RawMessage]{}, false
	}
	return config, true
}

// addEntries gives the array under key the entries it lacks, after those it holds, and returns them. An array
// that is not there is made of the entries; a value that is no array is the user's own, and stays.
func addEntries(config *manifest.Ordered[json.RawMessage], key string, entries []string) (added []string) {
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
		elements = append(elements, json.RawMessage(fsx.Quoted(entry)))
		added = append(added, entry)
	}
	config.Set(key, arrayOf(elements))
	return added
}

// elementsOf is the elements of a JSON array, each as it is written; false for a value that is no array, and for
// no value at all.
func elementsOf(written json.RawMessage) (elements []json.RawMessage, isArray bool) {
	if !startsWith(written, '[') || json.Unmarshal(written, &elements) != nil {
		return nil, false
	}
	return elements, true
}

// stringOf is the string a JSON value is; false for a value of another kind.
func stringOf(written json.RawMessage) (text string, isString bool) {
	if !startsWith(written, '"') || json.Unmarshal(written, &text) != nil {
		return "", false
	}
	return text, true
}

// startsWith reports whether the first character of JSON text, after its white space, is first: what tells an
// object, an array and a string from every other value.
func startsWith(text []byte, first byte) bool {
	text = bytes.TrimLeft(text, jsonSpace)
	return len(text) > 0 && text[0] == first
}

// stringsOf is the strings of a JSON array; false for a value that is no array, and for an array with an element
// that is no string.
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

// arrayOf is a JSON array of elements, each as it is written.
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

// writeLuarc writes the members as the project's .luarc.json.
func writeLuarc(root string, config manifest.Ordered[json.RawMessage]) error {
	text, err := laidOut(config)
	if err != nil {
		return err
	}
	return writeFile(root, luarcFile, text)
}

// laidOut is the members as the text of a file: an object with a member on each line, two spaces for each depth,
// and a final line break. A key is written with the escapes it needs and no other. A value keeps the text of each
// of its tokens, and the white space between them is laid out with the rest.
func laidOut(config manifest.Ordered[json.RawMessage]) ([]byte, error) {
	var onOneLine bytes.Buffer
	onOneLine.WriteByte('{')
	for key, value := range config.All() {
		if onOneLine.Len() > 1 {
			onOneLine.WriteByte(',')
		}
		onOneLine.WriteString(fsx.Quoted(key))
		onOneLine.WriteByte(':')
		onOneLine.Write(value)
	}
	onOneLine.WriteByte('}')
	var text bytes.Buffer
	// A failure here is passed on as it is: the keys are quoted and the values were read as JSON, so text that
	// cannot be laid out is a mistake in Moonwell and nothing the user can put right.
	if err := json.Indent(&text, onOneLine.Bytes(), "", "  "); err != nil {
		return nil, err
	}
	text.WriteByte('\n')
	return text.Bytes(), nil
}

// onDisk is where a file of the project is; file is its path from the project folder, with "/".
func onDisk(root, file string) string {
	return filepath.Join(root, filepath.FromSlash(file))
}

// readIfThere reads a file of the project. found is false, without an error, when there is none.
func readIfThere(root, file string) (data []byte, found bool, err error) {
	if data, found, err = fsx.ReadIfThere(onDisk(root, file)); err != nil {
		return nil, false, errNotRead(file, err)
	}
	return data, found, nil
}

// writeFile writes a file of the project, and makes the folder it is in.
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

// ---- errors ----

// errLinkToNothing is the refusal of an editor file of the project that is a link to a file that is not there, by
// its path from the project folder.
func errLinkToNothing(file string) error {
	return &diag.Error{
		Msg:  file + " is a link to a file that does not exist.",
		File: file,
		Hint: "Remove the link, or create the file it leads to, then run setup again.",
	}
}

// errNotRead is the failure to read an editor file of the project, by its path from the project folder.
func errNotRead(file string, cause error) error {
	return errFile("Reading", file, cause)
}

// errNotWritten is the failure to write an editor file of the project, by its path from the project folder.
func errNotWritten(file string, cause error) error {
	return errFile("Writing", file, cause)
}

// errFile is the failure of an action on an editor file of the project, with what the user can do about it.
func errFile(action, file string, cause error) error {
	return &diag.Error{
		Msg:  action + " " + file + " failed: " + fsx.Reason(cause),
		File: file,
		Hint: "Close any program that has " + file + " open and check that it is a file you can read and write, then run " +
			"setup again.",
		Cause: cause,
	}
}
