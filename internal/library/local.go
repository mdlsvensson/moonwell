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

func syncLocal(root string, dirs libraryDirs, path, dir, manifestName string) (shipsAssets bool, err error) {
	source, err := findSourceDirs(root, dirs, path, dir, manifestName)
	if err != nil {
		return false, err
	}
	content, err := readLocal(dirs.key, source, manifestName)
	if err != nil {
		return false, err
	}
	stamp := archiveFile{stampFile, []byte(localStampText(source.modules))}
	writeAssets, writeModules, err := planCopies(root, dirs, content, stamp)
	if err != nil {
		return false, err
	}
	if err := removeForeignStamp(dirs, stamp.data); err != nil {
		return false, err
	}
	if err := writeAssets(); err != nil {
		return false, err
	}
	return content.shipsAssets, writeModules()
}

func planCopies(root string, dirs libraryDirs, content libraryContent, stamp archiveFile) (writeAssets, writeModules func() error, err error) {
	modules, err := newMirror(root, modulesDirName(dirs.key), append(slices.Clone(content.modules), stamp))
	if err != nil {
		return nil, nil, err
	}
	if !content.shipsAssets {
		return func() error { return removeAssets(dirs) }, modules.write, nil
	}
	assets, err := newMirror(root, assetsDirName(dirs.key), content.assets)
	if err != nil {
		return nil, nil, err
	}
	return assets.write, modules.write, nil
}

func removeForeignStamp(dirs libraryDirs, ownStamp []byte) error {
	existing, err := os.ReadFile(filepath.Join(dirs.modules, stampFile))
	if err != nil || bytes.Equal(existing, ownStamp) {
		return nil
	}
	return removeStamp(dirs)
}

type sourceDirs struct {
	modules string
	assets  string
}

func findSourceDirs(root string, dirs libraryDirs, path, dir, manifestName string) (sourceDirs, error) {
	baseDir, err := resolveBaseDir(root, path)
	if err != nil {
		return sourceDirs{}, errUnreadableLibrary(dirs.key, path, manifestName, err)
	}
	libraryFilePath := filepath.Join(baseDir, File)
	libraryFile, err := readLibraryFile(dirs.key, libraryFilePath)
	if err != nil {
		return sourceDirs{}, err
	}
	source := relativeDirsOf(dir, libraryFile).resolve(baseDir)
	switch {
	case !fsx.IsDir(source.modules):
		return sourceDirs{}, errNoModuleFolder(dirs.key, source.modules, manifestName)
	case fsx.IsWithin(filepath.Dir(dirs.modules), source.modules):
		return sourceDirs{}, errHoldsTheLibraries(dirs.key, source.modules, manifestName)
	case source.assets != "" && !fsx.IsDir(source.assets):
		return sourceDirs{}, errNoAssetsFolder(dirs.key, source.assets, libraryFilePath)
	}
	return source, nil
}

func resolveBaseDir(root, path string) (string, error) {
	return filepath.Abs(fsx.ResolvePath(root, path))
}

type relativeDirs struct {
	modules string
	assets  string
}

func relativeDirsOf(dir string, libraryFile LibraryFile) relativeDirs {
	dirs := relativeDirs{modules: dir}
	if dir == "" && libraryFile.Dir != nil {
		dirs.modules = *libraryFile.Dir
	}
	if libraryFile.Assets != nil {
		dirs.assets = *libraryFile.Assets
	}
	return dirs
}

func (d relativeDirs) resolve(base string) sourceDirs {
	source := sourceDirs{modules: fsx.ResolvePath(base, d.modules)}
	if d.assets != "" {
		source.assets = fsx.ResolvePath(base, d.assets)
	}
	return source
}

func readLibraryFile(key, fullPath string) (LibraryFile, error) {
	content, found, err := fsx.ReadFileIfExists(fullPath)
	if err != nil && fsx.IsDir(filepath.Dir(fullPath)) {
		return LibraryFile{}, errUnreadableLibraryFile(key, fullPath, err)
	}
	return parseLibraryFile(key, content, found, fullPath)
}

func readLocal(key string, source sourceDirs, manifestName string) (libraryContent, error) {
	content := libraryContent{shipsAssets: source.assets != "", isLocal: true}
	var err error
	if content.modules, err = readFilesBelow(source.modules, isModule, source.assets); err == nil && content.shipsAssets {
		content.assets, err = readFilesBelow(source.assets, anyFile, "")
	}
	if err != nil {
		return libraryContent{}, errUnreadableLibrary(key, source.modules, manifestName, err)
	}
	return content, content.checkUsable(key, manifestName)
}

func isModule(name string) bool {
	return strings.HasSuffix(name, ".yue") || strings.HasSuffix(name, ".lua")
}

func anyFile(string) bool { return true }

func readFilesBelow(dir string, include func(name string) bool, skipDir string) ([]archiveFile, error) {
	names, err := listFilesBelow(dir, "", include, skipDir)
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

func listFilesBelow(dir, prefix string, include func(name string) bool, skipDir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, entry := range entries {
		name, fullPath := entry.Name(), filepath.Join(dir, entry.Name())
		switch {
		case strings.HasPrefix(name, "."):
		case entry.IsDir() && skipDir != "" && fsx.IsWithin(fullPath, skipDir):
		case entry.IsDir():
			nested, err := listFilesBelow(fullPath, prefix+name+"/", include, skipDir)
			if err != nil {
				return nil, err
			}
			files = append(files, nested...)
		case include(name):
			files = append(files, prefix+name)
		}
	}
	return files, nil
}

func errNoModuleFolder(key, dir, manifestName string) error {
	return &diag.Error{
		Msg:  "Library " + key + ": " + dir + " is not a folder.",
		File: manifestName,
		Hint: "Set the library's path (and dir) to a folder that holds its modules.",
	}
}

func errHoldsTheLibraries(key, dir, manifestName string) error {
	return &diag.Error{
		Msg:  "Library " + key + ": " + dir + " contains this project's " + ModulesDir + ".",
		File: manifestName,
		Hint: "Point the library's path (and dir) at the folder that holds its modules, not at the project.",
	}
}

func errNoAssetsFolder(key, dir, displayPath string) error {
	return &diag.Error{
		Msg:  "Library " + key + ": " + dir + " is not a folder.",
		File: displayPath,
		Hint: "Create the folder, or fix assets in the library's " + File + ".",
	}
}

func errUnreadableLibraryFile(key, displayPath string, cause error) error {
	return &diag.Error{
		Msg:   "Reading " + File + " of library " + key + " failed: " + describeError(cause),
		File:  displayPath,
		Hint:  "Close programs that have the file open, and check that it is a file that can be read.",
		Cause: cause,
	}
}

func errUnreadableLibrary(key, dir, manifestName string, cause error) error {
	return &diag.Error{
		Msg:   "Reading library " + key + " from " + dir + " failed: " + describeError(cause),
		File:  manifestName,
		Hint:  "Check the library's path and that its files can be read.",
		Cause: cause,
	}
}
