package library

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/fsx"
)

// syncLocal copies a local library, the folder at path, into its two folders: its modules, which are the .yue and
// .lua files below its module folder, and, when its moonwell-library.json names a folder of them, every file for
// the map. A file is written only when its bytes changed, and every other file of the two folders is removed. The
// library's own folder is read and never written. It reports whether the library ships files for the map.
func syncLocal(root string, at folders, path, dir, manifestFile string) (shipsAssets bool, err error) {
	from, err := sourcesOf(root, at, path, dir, manifestFile)
	if err != nil {
		return false, err
	}
	kept, err := readLocal(at.key, from, manifestFile)
	if err != nil {
		return false, err
	}
	stamp := file{stampFile, []byte(stampOfFolder(from.modules))}
	writeAssets, writeModules, err := planCopies(root, at, kept, stamp)
	if err != nil {
		return false, err
	}
	if err := dropOtherStamp(at, stamp.data); err != nil {
		return false, err
	}
	// The files for the map first, and the modules, whose folder holds the stamp, last.
	if err := writeAssets(); err != nil {
		return false, err
	}
	return kept.shipsAssets, writeModules()
}

// planCopies plans the two folders of a local library, and returns what writes each. Both are planned before
// either is written: a link in one of them is refused with nothing written, and no stamp removed.
func planCopies(root string, at folders, kept shipped, stamp file) (writeAssets, writeModules func() error, err error) {
	modules, err := mirrorOf(root, modulesOf(at.key), append(slices.Clone(kept.modules), stamp))
	if err != nil {
		return nil, nil, err
	}
	if !kept.shipsAssets {
		return func() error { return removeAssets(at) }, modules.write, nil
	}
	assets, err := mirrorOf(root, assetsOf(at.key), kept.assets)
	if err != nil {
		return nil, nil, err
	}
	return assets.write, modules.write, nil
}

// dropOtherStamp removes the stamp of the library's module folder when it is not own, the stamp of this local
// library: the stamp of a tag, or of another folder. While the two folders are written they then hold no entry,
// so a copy that stops halfway is not taken for the tag's files when the manifest names the tag again. The
// library's own stamp stays, and a library that did not change writes nothing.
func dropOtherStamp(at folders, own []byte) error {
	held, err := os.ReadFile(filepath.Join(at.modules, stampFile))
	if err != nil || bytes.Equal(held, own) {
		return nil
	}
	return dropStamp(at)
}

// ---- reading the library ----

// sources is the folders a local library is read from.
type sources struct {
	modules string // where the names of its modules start
	assets  string // its files for the map; "" when it names no such folder
}

// sourcesOf finds the folders of the local library at path, and refuses a library that cannot be copied from
// them. Where the folders are is said by three steps that Locals takes too, so that a library is watched where
// it is read: baseOf, describedAt and namedBy.
func sourcesOf(root string, at folders, path, dir, manifestFile string) (sources, error) {
	base, err := baseOf(root, path)
	if err != nil {
		return sources{}, errUnreadableLibrary(at.key, path, manifestFile, err)
	}
	libraryFile := filepath.Join(base, File)
	described, err := describedAt(at.key, libraryFile)
	if err != nil {
		return sources{}, err
	}
	from := namedBy(dir, described).below(base)
	switch {
	case !fsx.IsDir(from.modules):
		return sources{}, errNoModuleFolder(at.key, from.modules, manifestFile)
	case fsx.IsWithin(filepath.Dir(at.modules), from.modules):
		return sources{}, errHoldsTheLibraries(at.key, from.modules, manifestFile)
	case from.assets != "" && !fsx.IsDir(from.assets):
		return sources{}, errNoAssetsFolder(at.key, from.assets, libraryFile)
	}
	return from, nil
}

// baseOf is the folder of the local library at path, which is relative to the project folder or absolute.
func baseOf(root, path string) (string, error) {
	return filepath.Abs(fsx.Resolve(root, path))
}

// named is the two folders of a local library as they are written: each from the library's folder.
type named struct {
	modules string // "" for the library's folder itself
	assets  string // "" when the library names no folder of files for the map
}

// namedBy is the folders that the manifest's dir and the library's own file name. The module folder is the
// manifest's dir, else the one the library's own file names, else the library's folder. The folder of the files
// for the map is the library's file's alone.
func namedBy(dir string, described Described) named {
	folders := named{modules: dir}
	if dir == "" && described.Dir != nil {
		folders.modules = *described.Dir
	}
	if described.Assets != nil {
		folders.assets = *described.Assets
	}
	return folders
}

// below is the named folders on disk, for a library whose folder is base.
func (n named) below(base string) sources {
	from := sources{modules: fsx.Resolve(base, n.modules)}
	if n.assets != "" {
		from.assets = fsx.Resolve(base, n.assets)
	}
	return from
}

// describedAt is what a local library says of itself in libraryFile. A library without the file is the usual
// one, and a library whose folder is no folder has none either: sourcesOf refuses that one by its module folder.
// A file that is there and cannot be read is refused: taken for none, the library would be copied without the
// dir and the assets its file names, and without a word.
func describedAt(key, libraryFile string) (Described, error) {
	content, found, err := fsx.ReadIfThere(libraryFile)
	if err != nil && fsx.IsDir(filepath.Dir(libraryFile)) {
		return Described{}, errUnreadableLibraryFile(key, libraryFile, err)
	}
	return ParseFile(key, content, found, libraryFile)
}

// readLocal reads the files that are kept of a local library. The folder of the files for the map holds no
// modules, also when it lies inside the module folder.
func readLocal(key string, from sources, manifestFile string) (shipped, error) {
	kept := shipped{shipsAssets: from.assets != "", local: true}
	var err error
	if kept.modules, err = readBelow(from.modules, isModule, from.assets); err == nil && kept.shipsAssets {
		kept.assets, err = readBelow(from.assets, anyFile, "")
	}
	if err != nil {
		return shipped{}, errUnreadableLibrary(key, from.modules, manifestFile, err)
	}
	return kept, kept.refuseUnusable(key, manifestFile)
}

// isModule reports whether a file of the name is a module: a YueScript or a Lua file.
func isModule(name string) bool {
	return strings.HasSuffix(name, ".yue") || strings.HasSuffix(name, ".lua")
}

func anyFile(string) bool { return true }

// readBelow reads the files below dir whose name is wanted, each under its path from dir with "/".
func readBelow(dir string, wanted func(name string) bool, skip string) ([]file, error) {
	names, err := listBelow(dir, "", wanted, skip)
	if err != nil {
		return nil, err
	}
	files := make([]file, len(names))
	for i, name := range names {
		data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(name)))
		if err != nil {
			return nil, err
		}
		files[i] = file{name, data}
	}
	return files, nil
}

// listBelow lists the files below dir whose name is wanted, each as prefix and its path from dir with "/". It
// takes no file and enters no folder whose name starts with "." (.git, .DS_Store), and it does not enter the
// folder skip, when one is given. A link is no folder: it is listed as a file when its name is wanted.
func listBelow(dir, prefix string, wanted func(name string) bool, skip string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, entry := range entries {
		name, onDisk := entry.Name(), filepath.Join(dir, entry.Name())
		switch {
		case strings.HasPrefix(name, "."):
			// Passed over, with everything in it.
		case entry.IsDir() && skip != "" && fsx.IsWithin(onDisk, skip):
			// Passed over: the folder skip, or a folder inside it.
		case entry.IsDir():
			inner, err := listBelow(onDisk, prefix+name+"/", wanted, skip)
			if err != nil {
				return nil, err
			}
			files = append(files, inner...)
		case wanted(name):
			files = append(files, prefix+name)
		}
	}
	return files, nil
}

// ---- writing its copy ----

// mirror is a folder of the project that is made to hold exactly the files of a local library, the stamp among
// them where the folder has one.
type mirror struct {
	label   string   // the folder, from the project folder
	folder  string   // the folder on disk
	files   []file   // what it holds afterwards
	targets []string // where each of the files lies on disk
	anew    bool     // the folder is removed and made again: something in it lies where a file cannot be written
}

// mirrorOf plans the mirror of files in the folder label of the project at root. Nothing is written: a link at
// the folder, at one of the files or on the way to one is refused here.
func mirrorOf(root, label string, files []file) (mirror, error) {
	planned := mirror{label: label, files: files, targets: make([]string, len(files))}
	var err error
	if planned.folder, err = fsx.Inside(root, label); err != nil {
		return mirror{}, err
	}
	planned.anew = liesInTheWay(planned.folder, files)
	for i, f := range files {
		// A file on the way to a file's place is no failure here: fsx.Inside takes it for a place that nothing
		// is at, and the folder is then made anew. Any other failure to look at the place is refused.
		if planned.targets[i], err = fsx.Inside(root, label+"/"+f.name); err != nil {
			return mirror{}, err
		}
	}
	return planned, nil
}

// liesInTheWay reports whether the folder holds something where a file of the library cannot be written: a file
// in the place of the folder itself or of a folder on the way to a file, or a folder in the place of a file. A
// library gets there when a file of it becomes a folder of the same name, or the reverse. A link is none of
// these: it is refused where the path is reached.
func liesInTheWay(folder string, files []file) bool {
	if info, err := fsx.Lstat(folder); err == nil && info != nil && !info.IsDir() && !fsx.IsLink(info) {
		return true
	}
	for _, f := range files {
		path, segments := folder, strings.Split(f.name, "/")
		for i, segment := range segments {
			path = filepath.Join(path, segment)
			info, err := fsx.Lstat(path)
			if err != nil || info == nil || fsx.IsLink(info) {
				break
			}
			if isFile := i == len(segments)-1; info.IsDir() == isFile {
				return true
			}
		}
	}
	return false
}

// write makes the folder hold the files: a file whose bytes changed is written, and every file that is not among
// them is removed. Where that cannot give the library's own names, the folder is removed and every file written
// again: the folder is the program's own, and module names tell letter case apart.
func (m mirror) write() error {
	anew := m.anew
	if !anew {
		var err error
		if anew, err = m.writeChanged(); err != nil {
			return errUnwritable(m.label, err)
		}
	}
	if !anew {
		return nil
	}
	if err := writeAnew(m.folder, m.files); err != nil {
		return errUnwritable(m.label, err)
	}
	// A library without files has its folder all the same.
	if err := os.MkdirAll(m.folder, 0o777); err != nil {
		return errUnwritable(m.label, err)
	}
	return nil
}

// writeChanged writes the files whose bytes changed, and removes every file of the folder that is not among
// them. It removes nothing, and says so, when the folder spells a file in another letter case than the library
// does: where letter case is ignored, the file was written under the folder's spelling, and removing that name
// would remove the file. Its failures are the system's.
func (m mirror) writeChanged() (spelledAnother bool, err error) {
	for i, f := range m.files {
		if _, err := fsx.WriteIfChanged(m.targets[i], string(f.data)); err != nil {
			return false, err
		}
	}
	if err := os.MkdirAll(m.folder, 0o777); err != nil {
		return false, err
	}
	existing, err := fsx.ListFiles(m.folder)
	if err != nil {
		return false, err
	}
	others, spelledAnother := m.others(existing)
	if spelledAnother {
		return true, nil
	}
	for _, name := range others {
		if err := os.Remove(filepath.Join(m.folder, filepath.FromSlash(name))); err != nil {
			return false, err
		}
	}
	return false, nil
}

// others is the names among existing, as the folder spells them, that are none of the files; and whether one of
// them is a file in another letter case.
func (m mirror) others(existing []string) (others []string, spelledAnother bool) {
	kept, folded := map[string]bool{}, map[string]bool{}
	for _, f := range m.files {
		kept[f.name], folded[strings.ToLower(f.name)] = true, true
	}
	for _, name := range existing {
		switch {
		case kept[name]:
		case folded[strings.ToLower(name)]:
			return nil, true
		default:
			others = append(others, name)
		}
	}
	return others, false
}

// ---- errors ----

func errNoModuleFolder(key, folder, manifestFile string) error {
	return &diag.Error{
		Msg:  "Library " + key + ": " + folder + " is not a folder.",
		File: manifestFile,
		Hint: "Set the library's path (and dir) to a folder that holds its modules.",
	}
}

func errHoldsTheLibraries(key, folder, manifestFile string) error {
	return &diag.Error{
		Msg:  "Library " + key + ": " + folder + " contains this project's " + ModulesDir + ".",
		File: manifestFile,
		Hint: "Point the library's path (and dir) at the folder that holds its modules, not at the project.",
	}
}

func errNoAssetsFolder(key, folder, libraryFile string) error {
	return &diag.Error{
		Msg:  "Library " + key + ": " + folder + " is not a folder.",
		File: libraryFile,
		Hint: "Create the folder, or fix assets in the library's " + File + ".",
	}
}

// errUnreadableLibraryFile is the failure to read a local library's own file, which is there: one that another
// program holds, that may not be read, or that is a folder.
func errUnreadableLibraryFile(key, libraryFile string, cause error) error {
	return &diag.Error{
		Msg:   "Reading " + File + " of library " + key + " failed: " + reasonOf(cause),
		File:  libraryFile,
		Hint:  "Close programs that have the file open, and check that it is a file that can be read.",
		Cause: cause,
	}
}

func errUnreadableLibrary(key, folder, manifestFile string, cause error) error {
	return &diag.Error{
		Msg:   "Reading library " + key + " from " + folder + " failed: " + reasonOf(cause),
		File:  manifestFile,
		Hint:  "Check the library's path and that its files can be read.",
		Cause: cause,
	}
}
