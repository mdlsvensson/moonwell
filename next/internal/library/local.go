package library

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/mdlsvensson/moonwell/next/internal/diag"
	"github.com/mdlsvensson/moonwell/next/internal/fsx"
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
	// Both folders are planned before either is written: a link in one of them is refused with nothing written.
	modules, err := mirrorOf(root, modulesOf(at.key), kept.modules, stampOfFolder(from.modules))
	if err != nil {
		return false, err
	}
	writeAssets := func() error { return removeAssets(at) }
	if kept.shipsAssets {
		assets, err := mirrorOf(root, assetsOf(at.key), kept.assets, "")
		if err != nil {
			return false, err
		}
		writeAssets = assets.write
	}
	// The files for the map first, and the modules, whose folder holds the stamp, last.
	if err := writeAssets(); err != nil {
		return false, err
	}
	return kept.shipsAssets, modules.write()
}

// ---- reading the library ----

// sources is the folders a local library is read from.
type sources struct {
	modules string // where the names of its modules start
	assets  string // its files for the map; "" when it names no such folder
}

// sourcesOf finds the folders of the local library at path, which is relative to the project folder or absolute.
// The module folder is the manifest's dir, else the one the library's own file names, else the library's folder.
func sourcesOf(root string, at folders, path, dir, manifestFile string) (sources, error) {
	base, err := filepath.Abs(fsx.Resolve(root, path))
	if err != nil {
		return sources{}, errUnreadableLibrary(at.key, path, manifestFile, err)
	}
	libraryFile := filepath.Join(base, File)
	described, err := describedAt(at.key, libraryFile)
	if err != nil {
		return sources{}, err
	}
	if dir == "" && described.Dir != nil {
		dir = *described.Dir
	}
	from := sources{modules: fsx.Resolve(base, dir)}
	switch {
	case !fsx.IsDir(from.modules):
		return sources{}, errNoModuleFolder(at.key, from.modules, manifestFile)
	case fsx.IsWithin(filepath.Dir(at.modules), from.modules):
		return sources{}, errHoldsTheLibraries(at.key, from.modules, manifestFile)
	}
	if described.Assets != nil {
		from.assets = fsx.Resolve(base, *described.Assets)
		if !fsx.IsDir(from.assets) {
			return sources{}, errNoAssetsFolder(at.key, from.assets, libraryFile)
		}
	}
	return from, nil
}

// describedAt is what a local library says of itself in libraryFile. A file that cannot be read counts as none:
// a library without the file is the usual one.
func describedAt(key, libraryFile string) (Described, error) {
	content, err := os.ReadFile(libraryFile)
	return ParseFile(key, content, err == nil, libraryFile)
}

// readLocal reads the files that are kept of a local library. The folder of the files for the map holds no
// modules, also when it lies inside the module folder.
func readLocal(key string, from sources, manifestFile string) (shipped, error) {
	kept := shipped{shipsAssets: from.assets != ""}
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

// mirror is a folder of the project that is made to hold exactly the files of a local library.
type mirror struct {
	label   string   // the folder, from the project folder
	folder  string   // the folder on disk
	files   []file   // what it holds afterwards
	targets []string // where each of the files lies on disk
	stamp   string   // the stamp it holds beside the files; "" for none
	stampAt string   // where the stamp lies on disk
}

// mirrorOf plans the mirror of files in the folder label of the project at root, with a stamp when one is
// given. Nothing is written: a link at the folder, at one of the files or on the way to one is refused here.
func mirrorOf(root, label string, files []file, stamp string) (mirror, error) {
	planned := mirror{label: label, files: files, stamp: stamp, targets: make([]string, len(files))}
	var err error
	if planned.folder, err = inProject(root, label, label); err != nil {
		return mirror{}, err
	}
	for i, f := range files {
		if planned.targets[i], err = inProject(root, label+"/"+f.name, label); err != nil {
			return mirror{}, err
		}
	}
	if stamp != "" {
		if planned.stampAt, err = inProject(root, label+"/"+stampFile, label); err != nil {
			return mirror{}, err
		}
	}
	return planned, nil
}

// write makes the folder hold the files and the stamp: a file whose bytes changed is written, every file that
// is not among them is removed, and the stamp is written last.
func (m mirror) write() error {
	if err := m.writeFiles(); err != nil {
		return errUnwritable(m.label, err)
	}
	return nil
}

// writeFiles does what write says. Its failures are the system's.
func (m mirror) writeFiles() error {
	kept := map[string]bool{}
	for i, f := range m.files {
		kept[f.name] = true
		if _, err := fsx.WriteIfChanged(m.targets[i], string(f.data)); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(m.folder, 0o777); err != nil {
		return err
	}
	existing, err := fsx.ListFiles(m.folder)
	if err != nil {
		return err
	}
	for _, name := range existing {
		if kept[name] || (m.stamp != "" && name == stampFile) {
			continue
		}
		if err := os.Remove(filepath.Join(m.folder, filepath.FromSlash(name))); err != nil {
			return err
		}
	}
	if m.stamp == "" {
		return nil
	}
	_, err = fsx.WriteIfChanged(m.stampAt, m.stamp)
	return err
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

func errUnreadableLibrary(key, folder, manifestFile string, cause error) error {
	return &diag.Error{
		Msg:   "Reading library " + key + " from " + folder + " failed: " + reasonOf(cause),
		File:  manifestFile,
		Hint:  "Check the library's path and that its files can be read.",
		Cause: cause,
	}
}
