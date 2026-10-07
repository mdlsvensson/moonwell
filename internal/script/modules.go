package script

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/fsx"
)

// Kind is the language of a module file.
type Kind string

const (
	Yue Kind = "yue"
	Lua Kind = "lua"
)

// builtins are the modules the runtime provides; no file may take their names.
var builtins = []string{"moonwell"}

// Library is a library's module folder: a path from the project folder, with "/".
type Library struct{ Key, Dir string }

// Source is a gameplay module on disk.
type Source struct {
	Name    string // the dotted name, such as "utils.timer"
	Path    string // from the project folder, with "/", such as "lua/utils/timer.lua"
	Kind    Kind
	Library string // the library's key; "" for the project's own
	// Text is a Lua module's text; a YueScript module is read when it is compiled. It is the bytes of the file,
	// without a byte order mark at the start and with a first line for a shell blanked. Nothing is decoded, so it
	// may hold bytes that are not UTF-8: it is not to be written as JSON or read character by character where
	// the bytes must stay.
	Text string
}

// Collect lists every module: YueScript under src/, Lua under lua/, then each library's, in that order and then
// by path. It fails on a dotted file or folder name, on a module file whose name or folder is not valid UTF-8, on
// two files with one name and on a file that takes a built-in module's name. src/ must exist. So the name of
// every module it lists is valid UTF-8, and so is its path, below a library's folder as the caller names it.
//
// The libraries are taken in the order given and are not sorted: the caller passes them in the order of their
// keys. A library's YueScript comes before its Lua; within a folder and a kind, the modules are in the order of
// their paths' bytes. lua/ and a library's folder may be missing, and then hold no modules.
//
// A link in the place of one of the folders, or on the way to it, is refused. Inside a folder a link is taken as
// the listing gives it: a link to a file that is named as a module is that module, and a Lua one is read through
// the link; a link to a folder is not entered, so the modules behind it are not found.
func Collect(root string, libraries []Library) ([]Source, error) {
	ofLibraries, err := libraryFolders("Collect", libraries)
	if err != nil {
		return nil, err
	}
	return collectFrom(root, append(ownFolders(), ofLibraries...))
}

// CollectLibraries lists the libraries' modules alone, as Collect lists them, with Collect's refusals among them.
// It does not look at src/ or lua/: a module of a library that has the name of one of the project is listed.
//
// So it lists the modules of the libraries of a project that Collect refuses for its own modules: one without
// src/, one with a fault in src/ or lua/, and one in which a module of the project and a module of a library
// answer to one name. What Collect refuses of a library is refused here in the same words: a dotted name, a
// name that is not valid UTF-8, a built-in module's name, a name that two modules of the libraries answer to,
// and a link at a library's folder or on the way to it.
func CollectLibraries(root string, libraries []Library) ([]Source, error) {
	searched, err := libraryFolders("CollectLibraries", libraries)
	if err != nil {
		return nil, err
	}
	return collectFrom(root, searched)
}

// collectFrom lists the modules of the folders searched that are there, in the order of the folders.
func collectFrom(root string, searched []folder) ([]Source, error) {
	folders, err := moduleFolders(root, searched)
	if err != nil {
		return nil, err
	}
	found := collected{byName: map[string]Source{}}
	for _, f := range folders {
		if err := found.addFolder(f); err != nil {
			return nil, err
		}
	}
	return found.sources, nil
}

// EntryName is the dotted module name of an entry file: "src/game/init.yue" is "game.init". The entry is a .yue
// file under src/; a backslash is read as a separator, and a "./" at the start is dropped.
func EntryName(entry string) (string, error) {
	file := strings.TrimPrefix(strings.ReplaceAll(entry, `\`, "/"), "./")
	stem, isYue := strings.CutSuffix(file, ".yue")
	under, inSrc := strings.CutPrefix(stem, "src/")
	if !isYue || !inSrc {
		return "", errNoEntryFile(entry)
	}
	return strings.ReplaceAll(under, "/", "."), nil
}

// folder is a folder that is searched for the module files of one kind.
type folder struct {
	dir      string // from the project folder, with "/", such as "src"
	path     string // where the folder is on disk
	kind     Kind
	library  string // the library's key; "" for the project's own
	required bool   // a project without this folder is refused
}

// moduleFolders is the folders among searched that are there, in the order they are searched.
func moduleFolders(root string, searched []folder) ([]folder, error) {
	var present []folder
	for _, f := range searched {
		path, found, err := folderAt(root, f.dir)
		switch {
		case err != nil:
			return nil, err
		case found:
			f.path = path
			present = append(present, f)
		case f.required:
			return nil, errNoSrc(root)
		}
	}
	return present, nil
}

// ownFolders is the folders a project may hold modules of its own in, in the order they are searched: src/ for
// YueScript, then lua/ for Lua.
func ownFolders() []folder {
	return []folder{{dir: "src", kind: Yue, required: true}, {dir: "lua", kind: Lua}}
}

// libraryFolders is the folders the libraries may hold modules in, in the order they are searched: each
// library's folder for YueScript and then for Lua. door names the function that was handed the libraries, for
// the error.
func libraryFolders(door string, libraries []Library) ([]folder, error) {
	var searched []folder
	for _, library := range libraries {
		dir, ok := fsx.RelPath(library.Dir)
		if library.Key == "" || !ok {
			// A plain error: the caller makes the libraries from what a sync of them returned, so a library
			// without a key, or with a folder that is no path below the project folder, is a mistake in Moonwell
			// and nothing the user can put right.
			return nil, fmt.Errorf("script.%s: library %q has the folder %q, which no library can have",
				door, library.Key, library.Dir)
		}
		searched = append(searched,
			folder{dir: dir, kind: Yue, library: library.Key}, folder{dir: dir, kind: Lua, library: library.Key})
	}
	return searched, nil
}

// folderAt finds a folder of modules on disk. found is false where there is no folder: nothing at all, a file, or
// a link that leads to no folder. A link at a folder that is there, or on the way to it, is refused.
//
// Whether a folder is there is asked first, and the way to it is walked only when one is: a link that leads to
// no folder is then no folder, and is not refused.
func folderAt(root, dir string) (path string, found bool, err error) {
	if !fsx.IsDir(filepath.Join(root, filepath.FromSlash(dir))) {
		return "", false, nil
	}
	path, err = fsx.Inside(root, dir)
	return path, err == nil, err
}

// collected is the modules found so far, in the order they were found, and which module answers to each name.
type collected struct {
	sources []Source
	byName  map[string]Source
}

// addFolder adds the modules of a folder in the order of their paths. Each file is named, claims its names and
// is read, in that order, before the next: the first fault of the first faulty file is the one reported.
func (c *collected) addFolder(f folder) error {
	files, err := f.moduleFiles()
	if err != nil {
		return err
	}
	for _, file := range files {
		source, err := f.source(file)
		if err != nil {
			return err
		}
		if err := c.claim(source); err != nil {
			return err
		}
		if source.Kind == Lua {
			if source.Text, err = fsx.ReadSource(filepath.Join(f.path, filepath.FromSlash(file)), source.Path); err != nil {
				return err
			}
		}
		c.sources = append(c.sources, source)
	}
	return nil
}

// moduleFiles is the folder's files of its kind, as paths from the folder with "/", sorted by bytes.
func (f folder) moduleFiles() ([]string, error) {
	files, err := fsx.ListFiles(f.path)
	if err != nil {
		return nil, errUnreadableFolder(f.failedAt(err), err)
	}
	outputs := f.compiledOutputs(files)
	extension := "." + string(f.kind)
	var modules []string
	for _, file := range files {
		if strings.HasSuffix(file, extension) && !outputs[file] {
			modules = append(modules, file)
		}
	}
	return modules, nil
}

// failedAt is the folder at which a listing of f failed, as a path from the project folder with "/": the folder
// below f that the system's error names, and f itself where the error names none.
func (f folder) failedAt(cause error) string {
	var failure *fs.PathError
	if !errors.As(cause, &failure) {
		return f.dir
	}
	below, err := filepath.Rel(f.path, failure.Path)
	if err != nil || below == "." || !filepath.IsLocal(below) {
		return f.dir
	}
	return f.dir + "/" + fsx.ToPosix(below)
}

// compiledOutputs is the files among a library's that are no Lua modules. In a library, YueScript and Lua share
// one folder, and a .lua beside a .yue of the same stem is that module's compiled output.
func (f folder) compiledOutputs(files []string) map[string]bool {
	outputs := map[string]bool{}
	if f.library == "" || f.kind != Lua {
		return outputs
	}
	for _, file := range files {
		if stem, isYue := strings.CutSuffix(file, ".yue"); isYue {
			outputs[stem+".lua"] = true
		}
	}
	return outputs
}

// source is the module of a file of the folder, not yet read. Its name is the file's path from the folder without
// the extension, with a dot for each "/": so no file or folder on that path may have a dot in its own name.
//
// The name is what a require asks for and what the bundle defines the module by, as a string in the map's script,
// so it is valid UTF-8: a file whose path from the folder is not, in its own name or in a folder's, is refused. A
// file system holds such a name where names are bytes, and where they are UTF-16 units, for half of a pair.
func (f folder) source(file string) (Source, error) {
	path := f.dir + "/" + file
	stem := strings.TrimSuffix(file, "."+string(f.kind))
	switch {
	case strings.Contains(stem, "."):
		return Source{}, errDottedName(path, f.library != "")
	case !utf8.ValidString(file):
		return Source{}, errNameNotUTF8(path, f.library != "")
	}
	return Source{Name: strings.ReplaceAll(stem, "/", "."), Path: path, Kind: f.kind, Library: f.library}, nil
}

// claim records the names a module answers to. A name that is a built-in module's, or that a module found
// earlier answers to, is refused.
func (c *collected) claim(source Source) error {
	for _, name := range claimedNames(source.Name) {
		if slices.Contains(builtins, name) {
			return errBuiltinName(name, source)
		}
		if other, taken := c.byName[name]; taken {
			return errTwoFiles(name, other, source)
		}
		c.byName[name] = source
	}
	return nil
}

// claimedNames are the names a module answers to: its own and, for `<parent>.init`, `<parent>` too.
func claimedNames(name string) []string {
	if parent, isInit := strings.CutSuffix(name, ".init"); isInit {
		return []string{name, parent}
	}
	return []string{name}
}

// ---- errors ----

// narrowDir ends a hint for a library's file, which is not the project's to rename.
const narrowDir = "narrow the library's `dir` in moonwell.pkl so it leaves this file out."

func errNoSrc(root string) error {
	return &diag.Error{Msg: "The src/ folder is missing.", File: root}
}

func errUnreadableFolder(dir string, cause error) error {
	return &diag.Error{
		Msg:   "Reading " + dir + "/ failed: " + fsx.Reason(cause),
		File:  dir,
		Hint:  "Check that the folder and everything in it can be read, then try again.",
		Cause: cause,
	}
}

func errDottedName(path string, inLibrary bool) error {
	advice := "rename the file or folder."
	if inLibrary {
		advice = narrowDir
	}
	return &diag.Error{
		Msg:  "Module file and folder names cannot contain dots.",
		File: path,
		Hint: "Dots separate module names in `import`; " + advice,
	}
}

// errNameNotUTF8 refuses a module file whose path, which is not valid UTF-8, is given as it is.
func errNameNotUTF8(path string, inLibrary bool) error {
	advice := "rename the file or folder."
	if inLibrary {
		advice = narrowDir
	}
	return &diag.Error{
		Msg:  "Module file and folder names must be valid UTF-8.",
		File: path,
		Hint: "A module is required by its name, which the map's script holds as text; " + advice,
	}
}

func errBuiltinName(name string, source Source) error {
	if source.Library != "" {
		return &diag.Error{
			Msg:  "Module " + name + " is built into Moonwell; " + source.Path + " takes its name.",
			File: source.Path,
			Hint: "`require` of a built-in name always loads the built-in module; " + narrowDir,
		}
	}
	return &diag.Error{
		Msg:  "Module " + name + " is built into Moonwell; rename " + source.Path + ".",
		File: source.Path,
		Hint: "`require` of a built-in name always loads the built-in module, never a project file.",
	}
}

// errTwoFiles refuses the name that two modules answer to: first, which was found earlier, and second.
func errTwoFiles(name string, first, second Source) error {
	hint := "Rename one of them: module names are shared by src/ and lua/."
	if first.Library != "" || second.Library != "" {
		hint = "Rename one of them, or narrow the library's `dir`: module names are shared by src/, lua/ and libraries."
	}
	return &diag.Error{
		Msg:  "Module " + name + " is defined by " + first.Path + " and " + second.Path + ".",
		File: second.Path,
		Hint: hint,
	}
}

func errNoEntryFile(entry string) error {
	return &diag.Error{Msg: "Entry '" + entry + "' must be a .yue file under src/.", Hint: "For example: src/main.yue"}
}
