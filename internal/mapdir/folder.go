package mapdir

import (
	"os"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/fsx"
)

type Folder struct {
	dir, displayPath string
	onDisk           *diskIndex
	readHashes       map[string]string
	changes          []Change
	changeIndex      map[string]int
	newDirs          map[string]string
}

func Open(dir, displayPath string) (*Folder, error) {
	index, err := scanDir(dir, displayPath)
	if err != nil {
		return nil, err
	}
	return &Folder{
		dir: dir, displayPath: displayPath, onDisk: index,
		readHashes: map[string]string{}, changeIndex: map[string]int{}, newDirs: map[string]string{},
	}, nil
}

func (f *Folder) Dir() string { return f.dir }

func (f *Folder) DisplayPath(path string) string {
	if path == "" {
		return f.displayPath
	}
	return f.displayPath + "/" + f.CanonicalPath(path)
}

func (f *Folder) Files() []string {
	var files []string
	for _, path := range f.onDisk.files {
		if f.HasFile(path) {
			files = append(files, path)
		}
	}
	for _, change := range f.changes {
		if !f.onDisk.hasFile(change.Path) {
			files = append(files, change.Path)
		}
	}
	return files
}

func (f *Folder) HasFile(path string) bool {
	if index, ok := f.changeIndex[Key(path)]; ok {
		return !f.changes[index].Remove
	}
	return f.onDisk.hasFile(path)
}

func (f *Folder) IsDir(path string) bool {
	_, ok := f.dirPath(Key(path))
	return ok
}

func (f *Folder) CanonicalPath(path string) string {
	key := Key(path)
	if canonical, ok := f.filePath(key); ok {
		return canonical
	}
	if canonical, ok := f.dirPath(key); ok {
		return canonical
	}
	return path
}

func (f *Folder) filePath(key string) (string, bool) {
	if index, ok := f.changeIndex[key]; ok {
		return f.changes[index].Path, true
	}
	canonical, ok := f.onDisk.filePaths[key]
	return canonical, ok
}

func (f *Folder) dirPath(key string) (string, bool) {
	if path, ok := f.onDisk.dirPaths[key]; ok {
		return path, true
	}
	path, ok := f.newDirs[key]
	return path, ok
}

func (f *Folder) Read(path string) (data []byte, found bool, err error) {
	key := Key(path)
	if index, ok := f.changeIndex[key]; ok {
		if f.changes[index].Remove {
			return nil, false, nil
		}
		return f.changes[index].Data, true, nil
	}
	canonical, ok := f.onDisk.filePaths[key]
	if !ok {
		return nil, false, nil
	}
	if data, err = readFileIn(f.dir, canonical); err != nil {
		return nil, false, errUnreadable(f.DisplayPath(canonical), err)
	}
	f.recordReadHash(key, data)
	return data, true, nil
}

func readFileIn(dir, path string) ([]byte, error) {
	file, err := fsx.SafeJoin(dir, path)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(file)
}

func (f *Folder) recordReadHash(key string, data []byte) {
	if _, exists := f.readHashes[key]; !exists {
		f.readHashes[key] = fsx.SHA256Hex(data)
	}
}

func errUnreadable(file string, cause error) error {
	return &diag.Error{
		Msg:   "Reading a map file failed: " + fsx.Reason(cause),
		File:  file,
		Cause: cause,
		Hint:  "Make sure this is a readable file, not a folder, and that no other program has it locked.",
	}
}
