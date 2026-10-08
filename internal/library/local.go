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
	if err := writeAssets(); err != nil {
		return false, err
	}
	return kept.shipsAssets, writeModules()
}

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

func dropOtherStamp(at folders, own []byte) error {
	held, err := os.ReadFile(filepath.Join(at.modules, stampFile))
	if err != nil || bytes.Equal(held, own) {
		return nil
	}
	return dropStamp(at)
}

type sources struct {
	modules string
	assets  string
}

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

func baseOf(root, path string) (string, error) {
	return filepath.Abs(fsx.ResolvePath(root, path))
}

type named struct {
	modules string
	assets  string
}

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

func (n named) below(base string) sources {
	from := sources{modules: fsx.ResolvePath(base, n.modules)}
	if n.assets != "" {
		from.assets = fsx.ResolvePath(base, n.assets)
	}
	return from
}

func describedAt(key, libraryFile string) (Described, error) {
	content, found, err := fsx.ReadFileIfExists(libraryFile)
	if err != nil && fsx.IsDir(filepath.Dir(libraryFile)) {
		return Described{}, errUnreadableLibraryFile(key, libraryFile, err)
	}
	return parseFile(key, content, found, libraryFile)
}

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

func isModule(name string) bool {
	return strings.HasSuffix(name, ".yue") || strings.HasSuffix(name, ".lua")
}

func anyFile(string) bool { return true }

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
		case entry.IsDir() && skip != "" && fsx.IsWithin(onDisk, skip):
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

type mirror struct {
	label   string
	folder  string
	files   []file
	targets []string
	anew    bool
}

func mirrorOf(root, label string, files []file) (mirror, error) {
	planned := mirror{label: label, files: files, targets: make([]string, len(files))}
	var err error
	if planned.folder, err = fsx.SafeJoinNoSymlinks(root, label); err != nil {
		return mirror{}, err
	}
	planned.anew = liesInTheWay(planned.folder, files)
	for i, f := range files {
		if planned.targets[i], err = fsx.SafeJoinNoSymlinks(root, label+"/"+f.name); err != nil {
			return mirror{}, err
		}
	}
	return planned, nil
}

func liesInTheWay(folder string, files []file) bool {
	if info, err := fsx.Lstat(folder); err == nil && info != nil && !info.IsDir() && !fsx.IsSymlink(info) {
		return true
	}
	for _, f := range files {
		path, segments := folder, strings.Split(f.name, "/")
		for i, segment := range segments {
			path = filepath.Join(path, segment)
			info, err := fsx.Lstat(path)
			if err != nil || info == nil || fsx.IsSymlink(info) {
				break
			}
			if isFile := i == len(segments)-1; info.IsDir() == isFile {
				return true
			}
		}
	}
	return false
}

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
	if err := os.MkdirAll(m.folder, 0o777); err != nil {
		return errUnwritable(m.label, err)
	}
	return nil
}

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
