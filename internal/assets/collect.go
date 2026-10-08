package assets

import (
	"errors"
	"io/fs"
	"path/filepath"
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

const ownFolder = "assets"

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

func (c *collector) addProjectAssets(root string, config manifest.Assets) error {
	folder, err := openDir(filepath.Join(root, ownFolder), ownFolder)
	if err != nil {
		return err
	}
	rules, err := parseExcludeRules(config.Exclude)
	if err != nil {
		return c.blameAssetsBlock(err)
	}
	files := sortedFiles(folder)
	mapped, err := c.resolveMappings(config.Paths, files, rules)
	if err != nil {
		return err
	}
	for _, source := range files {
		if isExcluded(source, rules) {
			continue
		}
		if err := c.addProjectFile(folder, source, mapped); err != nil {
			return err
		}
	}
	if err := c.checkNoNesting(); err != nil {
		return err
	}
	c.sortByTarget()
	return nil
}

type excludeRule struct {
	key   string
	isDir bool
}

func parseExcludeRules(exclude []string) ([]excludeRule, error) {
	var rules []excludeRule
	for _, value := range exclude {
		name, folder := value, false
		if strings.HasSuffix(value, "/") || strings.HasSuffix(value, `\`) {
			name, folder = value[:len(value)-1], true
		}
		path, ok := fsx.CleanRelPath(name)
		if !ok {
			return nil, errInvalidPath(name)
		}
		rules = append(rules, excludeRule{mapdir.Key(path), folder})
	}
	return rules, nil
}

func isExcluded(path string, rules []excludeRule) bool {
	key := mapdir.Key(path)
	return hasHiddenSegment(path) || slices.ContainsFunc(rules, func(r excludeRule) bool {
		return key == r.key || (r.isDir && strings.HasPrefix(key, r.key+"/"))
	})
}

func hasHiddenSegment(path string) bool {
	return strings.HasPrefix(path, ".") || strings.Contains(path, "/.")
}

func (c *collector) resolveMappings(paths manifest.OrderedMap[string], files []string, rules []excludeRule) (map[string]string, error) {
	present := map[string]bool{}
	for _, file := range files {
		present[mapdir.Key(file)] = true
	}
	mapped := map[string]string{}
	for source, target := range paths.All() {
		path, ok := fsx.CleanRelPath(source)
		if !ok {
			return nil, c.blameAssetsBlock(errInvalidPath(source))
		}
		key := mapdir.Key(path)
		_, twice := mapped[key]
		switch {
		case !present[key]:
			return nil, errNoSuchFile(c.manifestName, source)
		case isExcluded(path, rules):
			return nil, errExcluded(c.manifestName, source)
		case twice:
			return nil, errNamedTwice(c.manifestName, source)
		}
		var err error
		if mapped[key], err = parseTargetPath(target); err != nil {
			return nil, c.blameAssetsBlock(err)
		}
	}
	return mapped, nil
}

func (c *collector) blameAssetsBlock(err error) error {
	var failure *diag.Error
	if errors.As(err, &failure) {
		failure.File = c.manifestName
		failure.Hint += " " + blockHint
	}
	return err
}

func (c *collector) addProjectFile(folder *mapdir.Folder, source string, mapped map[string]string) error {
	target, isMapped := mapped[mapdir.Key(source)]
	if !isMapped {
		var err error
		if target, err = parseTargetPath(source); err != nil {
			return err
		}
	}
	if _, taken := c.byTarget[mapdir.Key(target)]; taken {
		return errSameTarget(c.manifestName, target)
	}
	return c.add(folder, Asset{Source: source, Target: target})
}

func (c *collector) addLibraryAssets(root string, library Library) error {
	displayPath := displayPathOf(root, library.Dir)
	if err := checkNotSymlink(library.Dir, displayPath); err != nil {
		return err
	}
	folder, err := openDir(library.Dir, displayPath)
	if err != nil {
		return blameLibrary(err, library.Key, displayPath)
	}
	for _, source := range sortedFiles(folder) {
		if hasHiddenSegment(source) {
			continue
		}
		if err := c.addLibraryFile(folder, library.Key, displayPath, source); err != nil {
			return err
		}
	}
	return nil
}

func displayPathOf(root, dir string) string {
	if below, err := filepath.Rel(root, dir); err == nil && filepath.IsLocal(below) {
		return filepath.ToSlash(below)
	}
	return filepath.ToSlash(dir)
}

func checkNotSymlink(dir, displayPath string) error {
	if info, err := fsx.Lstat(dir); err == nil && info != nil && fsx.IsSymlink(info) {
		return errLinkedFolder(dir, displayPath)
	}
	return nil
}

func (c *collector) addLibraryFile(folder *mapdir.Folder, library, displayPath, source string) error {
	target, err := parseTargetPath(source)
	if err != nil {
		return blameLibrary(err, library, displayPath)
	}
	if other, taken := c.byTarget[mapdir.Key(target)]; taken {
		return c.replaceAsset(other, library, source, target)
	}
	return blameLibrary(c.add(folder, Asset{Source: source, Library: library, Target: target}), library, displayPath)
}

func (c *collector) replaceAsset(other Asset, library, source, target string) error {
	if other.Library != "" {
		return errTwoLibraries(c.manifestName, other.Library, library, target)
	}
	c.replaced = append(c.replaced, "assets/"+other.Source+" replaces library "+library+"'s "+source)
	return nil
}

func blameLibrary(err error, library, displayPath string) error {
	var failure *diag.Error
	if !errors.As(err, &failure) || failure.Cause != nil {
		return err
	}
	return errInLibrary(library, displayPath, failure)
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

const blockHint = "Fix the assets block in moonwell.pkl."

func errNoSuchFile(manifestName, source string) error {
	return &diag.Error{
		Msg:  "assets.paths names a file that does not exist: assets/" + source,
		File: manifestName,
		Hint: blockHint,
	}
}

func errExcluded(manifestName, source string) error {
	return &diag.Error{Msg: "assets.paths names an excluded file: " + source, File: manifestName, Hint: blockHint}
}

func errNamedTwice(manifestName, source string) error {
	return &diag.Error{Msg: "assets.paths names " + source + " twice.", File: manifestName, Hint: blockHint}
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

func errTwoLibraries(manifestName, first, second, target string) error {
	return &diag.Error{
		Msg:  "Libraries " + first + " and " + second + " both import " + strings.ReplaceAll(target, "/", `\`) + ".",
		File: manifestName,
		Hint: "Drop one of the libraries, or put your own file at that path under assets/ to replace both.",
	}
}

func errLinkedFolder(dir, displayPath string) error {
	err := fsx.NewSymlinkError(dir)
	var failure *diag.Error
	if errors.As(err, &failure) {
		failure.File = displayPath
	}
	return err
}

func errInLibrary(library, displayPath string, failure *diag.Error) error {
	return &diag.Error{
		Msg:   "Library " + library + ": " + failure.Msg,
		File:  displayPath,
		Hint:  "Report it to the library's author, or use another version of the library.",
		Cause: failure,
	}
}

func errNotAFolder(displayPath string) error {
	return &diag.Error{
		Msg:  "Expected a folder: " + displayPath,
		File: displayPath,
		Hint: "Make " + displayPath + " a folder that holds the files to import, or remove the file.",
	}
}
