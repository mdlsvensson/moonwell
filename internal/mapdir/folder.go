package mapdir

import (
	"os"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/fsx"
)

func Key(path string) string {
	return strings.ToLower(slashed(path))
}

func slashed(path string) string { return strings.ReplaceAll(path, `\`, "/") }

func join(folder, name string) string {
	if folder == "" {
		return name
	}
	return folder + "/" + name
}

type Folder struct {
	dir, label string
	found      *listing
	hashes     map[string]string
	changes    []Change
	planned    map[string]int
	made       map[string]string
}

func Open(dir, label string) (*Folder, error) {
	found, err := scan(dir, label)
	if err != nil {
		return nil, err
	}
	return &Folder{
		dir: dir, label: label, found: found,
		hashes: map[string]string{}, planned: map[string]int{}, made: map[string]string{},
	}, nil
}

func (f *Folder) Dir() string { return f.dir }

func (f *Folder) Label(name string) string {
	if name == "" {
		return f.label
	}
	return f.label + "/" + f.Name(name)
}

func (f *Folder) Files() []string {
	var files []string
	for _, name := range f.found.files {
		if f.Has(name) {
			files = append(files, name)
		}
	}
	for _, change := range f.changes {
		if !f.found.has(change.Name) {
			files = append(files, change.Name)
		}
	}
	return files
}

func (f *Folder) Has(name string) bool {
	if at, ok := f.planned[Key(name)]; ok {
		return !f.changes[at].Remove
	}
	return f.found.has(name)
}

func (f *Folder) IsFolder(name string) bool {
	_, is := f.folder(Key(name))
	return is
}

func (f *Folder) Name(name string) string {
	key := Key(name)
	if spelled, ok := f.spelling(key); ok {
		return spelled
	}
	if spelled, ok := f.folder(key); ok {
		return spelled
	}
	return name
}

func (f *Folder) spelling(key string) (string, bool) {
	if at, ok := f.planned[key]; ok {
		return f.changes[at].Name, true
	}
	spelled, ok := f.found.names[key]
	return spelled, ok
}

func (f *Folder) Read(name string) (data []byte, found bool, err error) {
	key := Key(name)
	if at, ok := f.planned[key]; ok {
		if f.changes[at].Remove {
			return nil, false, nil
		}
		return f.changes[at].Bytes, true, nil
	}
	spelled, ok := f.found.names[key]
	if !ok {
		return nil, false, nil
	}
	if data, err = readBelow(f.dir, spelled); err != nil {
		return nil, false, errUnreadable(f.Label(spelled), err)
	}
	f.remember(key, data)
	return data, true, nil
}

func readBelow(dir, path string) ([]byte, error) {
	file, err := fsx.SafeJoin(dir, path)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(file)
}

func (f *Folder) remember(key string, data []byte) {
	if _, noted := f.hashes[key]; !noted {
		f.hashes[key] = fsx.SHA256Hex(data)
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
