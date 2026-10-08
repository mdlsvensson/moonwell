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
	Bytes   []byte
	Hash    string
}

type Library struct {
	Key string
	Dir string
}

const ownFolder = "assets"

func Collect(root string, config manifest.Assets, manifestFile string, libraries []Library) (assets []Asset,
	replaced []string, err error) {
	c := &collection{manifestFile: manifestFile, assets: []Asset{}, taken: map[string]Asset{}, replaced: []string{}}
	if err = c.addOwn(root, config); err != nil {
		return nil, nil, err
	}
	for _, library := range byKey(libraries) {
		if err = c.addLibrary(root, library); err != nil {
			return nil, nil, err
		}
	}
	if err = c.refuseNesting(); err != nil {
		return nil, nil, err
	}
	c.sortByTarget()
	return c.assets, c.replaced, nil
}

type collection struct {
	manifestFile string
	assets       []Asset
	taken        map[string]Asset
	replaced     []string
}

func byKey(libraries []Library) []Library {
	sorted := slices.Clone(libraries)
	slices.SortStableFunc(sorted, func(a, b Library) int { return strings.Compare(a.Key, b.Key) })
	return sorted
}

func (c *collection) addOwn(root string, config manifest.Assets) error {
	folder, err := open(filepath.Join(root, ownFolder), ownFolder)
	if err != nil {
		return err
	}
	rules, err := rulesOf(config.Exclude)
	if err != nil {
		return c.inBlock(err)
	}
	files := filesOf(folder)
	mapped, err := c.mappings(config.Paths, files, rules)
	if err != nil {
		return err
	}
	for _, source := range files {
		if leftOut(source, rules) {
			continue
		}
		if err := c.addOwnFile(folder, source, mapped); err != nil {
			return err
		}
	}
	if err := c.refuseNesting(); err != nil {
		return err
	}
	c.sortByTarget()
	return nil
}

type rule struct {
	key    string
	folder bool
}

func rulesOf(exclude []string) ([]rule, error) {
	var rules []rule
	for _, value := range exclude {
		name, folder := value, false
		if strings.HasSuffix(value, "/") || strings.HasSuffix(value, `\`) {
			name, folder = value[:len(value)-1], true
		}
		path, ok := fsx.CleanRelPath(name)
		if !ok {
			return nil, errInvalidPath(name)
		}
		rules = append(rules, rule{mapdir.Key(path), folder})
	}
	return rules, nil
}

func leftOut(path string, rules []rule) bool {
	key := mapdir.Key(path)
	return hasDotPart(path) || slices.ContainsFunc(rules, func(r rule) bool {
		return key == r.key || (r.folder && strings.HasPrefix(key, r.key+"/"))
	})
}

func hasDotPart(path string) bool {
	return strings.HasPrefix(path, ".") || strings.Contains(path, "/.")
}

func (c *collection) mappings(paths manifest.Ordered[string], files []string, rules []rule) (map[string]string, error) {
	present := map[string]bool{}
	for _, file := range files {
		present[mapdir.Key(file)] = true
	}
	mapped := map[string]string{}
	for source, target := range paths.All() {
		path, ok := fsx.CleanRelPath(source)
		if !ok {
			return nil, c.inBlock(errInvalidPath(source))
		}
		key := mapdir.Key(path)
		_, twice := mapped[key]
		switch {
		case !present[key]:
			return nil, errNoSuchFile(c.manifestFile, source)
		case leftOut(path, rules):
			return nil, errExcluded(c.manifestFile, source)
		case twice:
			return nil, errNamedTwice(c.manifestFile, source)
		}
		var err error
		if mapped[key], err = targetPath(target); err != nil {
			return nil, c.inBlock(err)
		}
	}
	return mapped, nil
}

func (c *collection) inBlock(err error) error {
	var failure *diag.Error
	if errors.As(err, &failure) {
		failure.File = c.manifestFile
		failure.Hint += " " + blockHint
	}
	return err
}

func (c *collection) addOwnFile(folder *mapdir.Folder, source string, mapped map[string]string) error {
	target, isMapped := mapped[mapdir.Key(source)]
	if !isMapped {
		var err error
		if target, err = targetPath(source); err != nil {
			return err
		}
	}
	if _, taken := c.taken[mapdir.Key(target)]; taken {
		return errSameTarget(c.manifestFile, target)
	}
	return c.add(folder, Asset{Source: source, Target: target})
}

func (c *collection) addLibrary(root string, library Library) error {
	label := labelOf(root, library.Dir)
	if err := refuseLinked(library.Dir, label); err != nil {
		return err
	}
	folder, err := open(library.Dir, label)
	if err != nil {
		return inLibrary(err, library.Key, label)
	}
	for _, source := range filesOf(folder) {
		if hasDotPart(source) {
			continue
		}
		if err := c.addShipped(folder, library.Key, label, source); err != nil {
			return err
		}
	}
	return nil
}

func labelOf(root, dir string) string {
	if below, err := filepath.Rel(root, dir); err == nil && filepath.IsLocal(below) {
		return filepath.ToSlash(below)
	}
	return filepath.ToSlash(dir)
}

func refuseLinked(dir, label string) error {
	if info, err := fsx.Lstat(dir); err == nil && info != nil && fsx.IsSymlink(info) {
		return errLinkedFolder(dir, label)
	}
	return nil
}

func (c *collection) addShipped(folder *mapdir.Folder, library, label, source string) error {
	target, err := targetPath(source)
	if err != nil {
		return inLibrary(err, library, label)
	}
	if other, taken := c.taken[mapdir.Key(target)]; taken {
		return c.replace(other, library, source, target)
	}
	return inLibrary(c.add(folder, Asset{Source: source, Library: library, Target: target}), library, label)
}

func (c *collection) replace(other Asset, library, source, target string) error {
	if other.Library != "" {
		return errTwoLibraries(c.manifestFile, other.Library, library, target)
	}
	c.replaced = append(c.replaced, "assets/"+other.Source+" replaces library "+library+"'s "+source)
	return nil
}

func inLibrary(err error, library, label string) error {
	var failure *diag.Error
	if !errors.As(err, &failure) || failure.Cause != nil {
		return err
	}
	return errInLibrary(library, label, failure)
}

func open(dir, label string) (*mapdir.Folder, error) {
	if info, err := fsx.Lstat(dir); err == nil && info != nil && !info.IsDir() && !fsx.IsSymlink(info) {
		return nil, errNotAFolder(label)
	}
	folder, err := mapdir.Open(dir, label)
	if _, expected := diag.FirstProblem(err); !expected && errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	return folder, err
}

func filesOf(folder *mapdir.Folder) []string {
	if folder == nil {
		return nil
	}
	return folder.Files()
}

func (c *collection) add(folder *mapdir.Folder, asset Asset) error {
	data, _, err := folder.Read(asset.Source)
	if err != nil {
		return err
	}
	asset.Bytes, asset.Hash = data, fsx.SHA256Hex(data)
	c.assets = append(c.assets, asset)
	c.taken[mapdir.Key(asset.Target)] = asset
	return nil
}

func (c *collection) refuseNesting() error {
	for _, asset := range c.assets {
		key := mapdir.Key(asset.Target)
		for end := strings.LastIndexByte(key, '/'); end >= 0; end = strings.LastIndexByte(key[:end], '/') {
			if _, taken := c.taken[key[:end]]; taken {
				return errNested(c.manifestFile, key, key[:end])
			}
		}
	}
	return nil
}

func (c *collection) sortByTarget() {
	slices.SortStableFunc(c.assets, func(a, b Asset) int {
		return strings.Compare(mapdir.Key(a.Target), mapdir.Key(b.Target))
	})
}

const blockHint = "Fix the assets block in moonwell.pkl."

func errNoSuchFile(manifestFile, source string) error {
	return &diag.Error{
		Msg:  "assets.paths names a file that does not exist: assets/" + source,
		File: manifestFile,
		Hint: blockHint,
	}
}

func errExcluded(manifestFile, source string) error {
	return &diag.Error{Msg: "assets.paths names an excluded file: " + source, File: manifestFile, Hint: blockHint}
}

func errNamedTwice(manifestFile, source string) error {
	return &diag.Error{Msg: "assets.paths names " + source + " twice.", File: manifestFile, Hint: blockHint}
}

func errSameTarget(manifestFile, target string) error {
	return &diag.Error{
		Msg:  "Two assets would be imported as " + target + " (target collision).",
		File: manifestFile,
		Hint: blockHint,
	}
}

func errNested(manifestFile, inner, outer string) error {
	return &diag.Error{
		Msg:  "Asset " + inner + " would sit inside the asset file " + outer + " (file/folder collision).",
		File: manifestFile,
		Hint: blockHint,
	}
}

func errTwoLibraries(manifestFile, first, second, target string) error {
	return &diag.Error{
		Msg:  "Libraries " + first + " and " + second + " both import " + strings.ReplaceAll(target, "/", `\`) + ".",
		File: manifestFile,
		Hint: "Drop one of the libraries, or put your own file at that path under assets/ to replace both.",
	}
}

func errLinkedFolder(dir, label string) error {
	err := fsx.NewSymlinkError(dir)
	var failure *diag.Error
	if errors.As(err, &failure) {
		failure.File = label
	}
	return err
}

func errInLibrary(library, label string, failure *diag.Error) error {
	return &diag.Error{
		Msg:   "Library " + library + ": " + failure.Msg,
		File:  label,
		Hint:  "Report it to the library's author, or use another version of the library.",
		Cause: failure,
	}
}

func errNotAFolder(label string) error {
	return &diag.Error{
		Msg:  "Expected a folder: " + label,
		File: label,
		Hint: "Make " + label + " a folder that holds the files to import, or remove the file.",
	}
}
