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

func syncLocal(root string, at libraryDirs, path, dir, manifestName string) (shipsAssets bool, err error) {
	from, err := findSourceDirs(root, at, path, dir, manifestName)
	if err != nil {
		return false, err
	}
	kept, err := readLocal(at.key, from, manifestName)
	if err != nil {
		return false, err
	}
	stamp := archiveFile{stampFile, []byte(localStampText(from.modules))}
	writeAssets, writeModules, err := planCopies(root, at, kept, stamp)
	if err != nil {
		return false, err
	}
	if err := removeForeignStamp(at, stamp.data); err != nil {
		return false, err
	}
	if err := writeAssets(); err != nil {
		return false, err
	}
	return kept.shipsAssets, writeModules()
}

func planCopies(root string, at libraryDirs, kept libraryContent, stamp archiveFile) (writeAssets, writeModules func() error, err error) {
	modules, err := newMirror(root, modulesDirName(at.key), append(slices.Clone(kept.modules), stamp))
	if err != nil {
		return nil, nil, err
	}
	if !kept.shipsAssets {
		return func() error { return removeAssets(at) }, modules.write, nil
	}
	assets, err := newMirror(root, assetsDirName(at.key), kept.assets)
	if err != nil {
		return nil, nil, err
	}
	return assets.write, modules.write, nil
}

func removeForeignStamp(at libraryDirs, own []byte) error {
	held, err := os.ReadFile(filepath.Join(at.modules, stampFile))
	if err != nil || bytes.Equal(held, own) {
		return nil
	}
	return removeStamp(at)
}

type sourceDirs struct {
	modules string
	assets  string
}

func findSourceDirs(root string, at libraryDirs, path, dir, manifestName string) (sourceDirs, error) {
	base, err := resolveBase(root, path)
	if err != nil {
		return sourceDirs{}, errUnreadableLibrary(at.key, path, manifestName, err)
	}
	libraryFile := filepath.Join(base, File)
	described, err := readLibraryFile(at.key, libraryFile)
	if err != nil {
		return sourceDirs{}, err
	}
	from := relativeDirsOf(dir, described).resolve(base)
	switch {
	case !fsx.IsDir(from.modules):
		return sourceDirs{}, errNoModuleFolder(at.key, from.modules, manifestName)
	case fsx.IsWithin(filepath.Dir(at.modules), from.modules):
		return sourceDirs{}, errHoldsTheLibraries(at.key, from.modules, manifestName)
	case from.assets != "" && !fsx.IsDir(from.assets):
		return sourceDirs{}, errNoAssetsFolder(at.key, from.assets, libraryFile)
	}
	return from, nil
}

func resolveBase(root, path string) (string, error) {
	return filepath.Abs(fsx.ResolvePath(root, path))
}

type relativeDirs struct {
	modules string
	assets  string
}

func relativeDirsOf(dir string, described LibraryFile) relativeDirs {
	folders := relativeDirs{modules: dir}
	if dir == "" && described.Dir != nil {
		folders.modules = *described.Dir
	}
	if described.Assets != nil {
		folders.assets = *described.Assets
	}
	return folders
}

func (d relativeDirs) resolve(base string) sourceDirs {
	from := sourceDirs{modules: fsx.ResolvePath(base, d.modules)}
	if d.assets != "" {
		from.assets = fsx.ResolvePath(base, d.assets)
	}
	return from
}

func readLibraryFile(key, libraryFile string) (LibraryFile, error) {
	content, found, err := fsx.ReadFileIfExists(libraryFile)
	if err != nil && fsx.IsDir(filepath.Dir(libraryFile)) {
		return LibraryFile{}, errUnreadableLibraryFile(key, libraryFile, err)
	}
	return parseLibraryFile(key, content, found, libraryFile)
}

func readLocal(key string, from sourceDirs, manifestName string) (libraryContent, error) {
	kept := libraryContent{shipsAssets: from.assets != "", local: true}
	var err error
	if kept.modules, err = readFilesBelow(from.modules, isModule, from.assets); err == nil && kept.shipsAssets {
		kept.assets, err = readFilesBelow(from.assets, anyFile, "")
	}
	if err != nil {
		return libraryContent{}, errUnreadableLibrary(key, from.modules, manifestName, err)
	}
	return kept, kept.checkUsable(key, manifestName)
}

func isModule(name string) bool {
	return strings.HasSuffix(name, ".yue") || strings.HasSuffix(name, ".lua")
}

func anyFile(string) bool { return true }

func readFilesBelow(dir string, wanted func(name string) bool, skip string) ([]archiveFile, error) {
	names, err := listFilesBelow(dir, "", wanted, skip)
	if err != nil {
		return nil, err
	}
	files := make([]archiveFile, len(names))
	for i, name := range names {
		data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(name)))
		if err != nil {
			return nil, err
		}
		files[i] = archiveFile{name, data}
	}
	return files, nil
}

func listFilesBelow(dir, prefix string, wanted func(name string) bool, skip string) ([]string, error) {
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
			inner, err := listFilesBelow(onDisk, prefix+name+"/", wanted, skip)
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

type dirMirror struct {
	displayPath string
	dir         string
	files       []archiveFile
	targets     []string
	isNew       bool
}

func newMirror(root, label string, files []archiveFile) (dirMirror, error) {
	planned := dirMirror{displayPath: label, files: files, targets: make([]string, len(files))}
	var err error
	if planned.dir, err = fsx.SafeJoinNoSymlinks(root, label); err != nil {
		return dirMirror{}, err
	}
	planned.isNew = isBlockedByFile(planned.dir, files)
	for i, f := range files {
		if planned.targets[i], err = fsx.SafeJoinNoSymlinks(root, label+"/"+f.name); err != nil {
			return dirMirror{}, err
		}
	}
	return planned, nil
}

func isBlockedByFile(folder string, files []archiveFile) bool {
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

func (m dirMirror) write() error {
	anew := m.isNew
	if !anew {
		var err error
		if anew, err = m.writeChanged(); err != nil {
			return errUnwritable(m.displayPath, err)
		}
	}
	if !anew {
		return nil
	}
	if err := writeFiles(m.dir, m.files); err != nil {
		return errUnwritable(m.displayPath, err)
	}
	if err := os.MkdirAll(m.dir, 0o777); err != nil {
		return errUnwritable(m.displayPath, err)
	}
	return nil
}

func (m dirMirror) writeChanged() (spelledAnother bool, err error) {
	for i, f := range m.files {
		if _, err := fsx.WriteIfChanged(m.targets[i], string(f.data)); err != nil {
			return false, err
		}
	}
	if err := os.MkdirAll(m.dir, 0o777); err != nil {
		return false, err
	}
	existing, err := fsx.ListFiles(m.dir)
	if err != nil {
		return false, err
	}
	others, spelledAnother := m.staleFiles(existing)
	if spelledAnother {
		return true, nil
	}
	for _, name := range others {
		if err := os.Remove(filepath.Join(m.dir, filepath.FromSlash(name))); err != nil {
			return false, err
		}
	}
	return false, nil
}

func (m dirMirror) staleFiles(existing []string) (others []string, spelledAnother bool) {
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

func errNoModuleFolder(key, folder, manifestName string) error {
	return &diag.Error{
		Msg:  "Library " + key + ": " + folder + " is not a folder.",
		File: manifestName,
		Hint: "Set the library's path (and dir) to a folder that holds its modules.",
	}
}

func errHoldsTheLibraries(key, folder, manifestName string) error {
	return &diag.Error{
		Msg:  "Library " + key + ": " + folder + " contains this project's " + ModulesDir + ".",
		File: manifestName,
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
		Msg:   "Reading " + File + " of library " + key + " failed: " + describeFetchError(cause),
		File:  libraryFile,
		Hint:  "Close programs that have the file open, and check that it is a file that can be read.",
		Cause: cause,
	}
}

func errUnreadableLibrary(key, folder, manifestName string, cause error) error {
	return &diag.Error{
		Msg:   "Reading library " + key + " from " + folder + " failed: " + describeFetchError(cause),
		File:  manifestName,
		Hint:  "Check the library's path and that its files can be read.",
		Cause: cause,
	}
}
