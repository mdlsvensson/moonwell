package assets

import (
	"errors"
	"io/fs"
	"slices"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/manifest"
	"github.com/mdlsvensson/moonwell/internal/mapdir"
)

type Asset struct {
	Source  string
	Library string
	Target  string
	Data    []byte
	Hash    string
}

type Library struct {
	Key string
	Dir string
}

func Collect(root string, config manifest.Assets, manifestName string, libraries []Library) (assets []Asset,
	replaced []string, err error) {
	c := &collector{manifestName: manifestName, assets: []Asset{}, byTarget: map[string]Asset{}, replaced: []string{}}
	if err = c.addProjectAssets(root, config); err != nil {
		return nil, nil, err
	}
	for _, library := range sortedByKey(libraries) {
		if err = c.addLibraryAssets(root, library); err != nil {
			return nil, nil, err
		}
	}
	if err = c.checkNoNesting(); err != nil {
		return nil, nil, err
	}
	c.sortByTarget()
	return c.assets, c.replaced, nil
}

type collector struct {
	manifestName string
	assets       []Asset
	byTarget     map[string]Asset
	replaced     []string
}

func sortedByKey(libraries []Library) []Library {
	sorted := slices.Clone(libraries)
	slices.SortStableFunc(sorted, func(a, b Library) int { return strings.Compare(a.Key, b.Key) })
	return sorted
}

func openDir(dir, displayPath string) (*mapdir.Folder, error) {
	if info, err := fsx.Lstat(dir); err == nil && info != nil && !info.IsDir() && !fsx.IsSymlink(info) {
		return nil, errNotAFolder(displayPath)
	}
	folder, err := mapdir.Open(dir, displayPath)
	if _, expected := diag.FirstProblem(err); !expected && errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	return folder, err
}

func sortedFiles(folder *mapdir.Folder) []string {
	if folder == nil {
		return nil
	}
	return folder.Files()
}

func (c *collector) add(folder *mapdir.Folder, asset Asset) error {
	data, _, err := folder.Read(asset.Source)
	if err != nil {
		return err
	}
	asset.Data, asset.Hash = data, fsx.SHA256Hex(data)
	c.assets = append(c.assets, asset)
	c.byTarget[mapdir.Key(asset.Target)] = asset
	return nil
}

func (c *collector) checkNoNesting() error {
	for _, asset := range c.assets {
		key := mapdir.Key(asset.Target)
		for end := strings.LastIndexByte(key, '/'); end >= 0; end = strings.LastIndexByte(key[:end], '/') {
			if _, taken := c.byTarget[key[:end]]; taken {
				return errNested(c.manifestName, key, key[:end])
			}
		}
	}
	return nil
}

func (c *collector) sortByTarget() {
	slices.SortStableFunc(c.assets, func(a, b Asset) int {
		return strings.Compare(mapdir.Key(a.Target), mapdir.Key(b.Target))
	})
}

func errSameTarget(manifestName, target string) error {
	return &diag.Error{
		Msg:  "Two assets would be imported as " + target + " (target collision).",
		File: manifestName,
		Hint: blockHint,
	}
}

func errNested(manifestName, inner, outer string) error {
	return &diag.Error{
		Msg:  "Asset " + inner + " would sit inside the asset file " + outer + " (file/folder collision).",
		File: manifestName,
		Hint: blockHint,
	}
}

func errNotAFolder(displayPath string) error {
	return &diag.Error{
		Msg:  "Expected a folder: " + displayPath,
		File: displayPath,
		Hint: "Make " + displayPath + " a folder that holds the files to import, or remove the file.",
	}
}
