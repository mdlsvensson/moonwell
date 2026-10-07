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

// Asset is a file to import, and the in-map path it is imported as.
type Asset struct {
	Source  string // its path under assets/, or under the library's folder, with "/"
	Library string // the library that ships it; empty for the map's own
	Target  string // the in-map path, with "/"
	Bytes   []byte
	Hash    string // the SHA-256 of Bytes, in lower-case hexadecimal
}

// Library is a library that ships files for the map.
type Library struct {
	Key string
	Dir string // the folder that holds those files
}

// ownFolder is the folder of a project that holds the map's own assets, and how errors name it.
const ownFolder = "assets"

// Collect returns the map's own assets and then the files the libraries ship, sorted by in-map path. The map's
// own file wins over a library's at the same in-map path, with a line in replaced saying so; two libraries at
// one path fail. manifestFile is the file that errors about the assets block name. Nothing is written.
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

// collection is the assets found so far.
type collection struct {
	manifestFile string
	assets       []Asset
	taken        map[string]Asset // by the key of an in-map path, the asset imported there
	replaced     []string
}

// byKey is the libraries in the order of their keys, so that what is found does not depend on the order given.
func byKey(libraries []Library) []Library {
	sorted := slices.Clone(libraries)
	slices.SortStableFunc(sorted, func(a, b Library) int { return strings.Compare(a.Key, b.Key) })
	return sorted
}

// ---- the map's own assets ----

// addOwn adds the files under assets/ that the manifest does not leave out, each under the path assets.paths
// gives it, else under its own. They are checked among themselves and sorted before a library's files follow.
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

// rule is one entry of assets.exclude: a file, or a folder with everything below it.
type rule struct {
	key    string
	folder bool
}

// rulesOf reads assets.exclude. An entry that ends in a separator names a folder.
func rulesOf(exclude []string) ([]rule, error) {
	var rules []rule
	for _, value := range exclude {
		name, folder := value, false
		if strings.HasSuffix(value, "/") || strings.HasSuffix(value, `\`) {
			name, folder = value[:len(value)-1], true
		}
		path, ok := fsx.RelPath(name)
		if !ok {
			return nil, errInvalidPath(name)
		}
		rules = append(rules, rule{mapdir.Key(path), folder})
	}
	return rules, nil
}

// leftOut reports whether the file at path is not imported: a rule names it or a folder it is in, or a part of its
// name starts with a dot.
func leftOut(path string, rules []rule) bool {
	key := mapdir.Key(path)
	return hasDotPart(path) || slices.ContainsFunc(rules, func(r rule) bool {
		return key == r.key || (r.folder && strings.HasPrefix(key, r.key+"/"))
	})
}

// hasDotPart reports whether a part of path, with "/", starts with a dot: .gitkeep, .git/config.
func hasDotPart(path string) bool {
	return strings.HasPrefix(path, ".") || strings.Contains(path, "/.")
}

// mappings is the in-map path assets.paths gives each file it names, by the file's key. A mapping names a file
// that is among files and is not left out, and no file has two.
func (c *collection) mappings(paths manifest.Ordered[string], files []string, rules []rule) (map[string]string, error) {
	present := map[string]bool{}
	for _, file := range files {
		present[mapdir.Key(file)] = true
	}
	mapped := map[string]string{}
	for source, target := range paths.All() {
		path, ok := fsx.RelPath(source)
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

// inBlock makes the refusal of a path that is written in the assets block the block's own: the manifest is its
// file, and the block's hint follows what the path's hint says. A path that is a file's own name is not written
// there, and its refusal stays as it is.
func (c *collection) inBlock(err error) error {
	var failure *diag.Error
	if errors.As(err, &failure) {
		failure.File = c.manifestFile
		failure.Hint += " " + blockHint
	}
	return err
}

// addOwnFile adds one of the map's own files, under the path its mapping gives, else under its own.
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

// ---- the libraries' files ----

// addLibrary adds the files a library ships, each at its path in the library's folder. A file whose in-map path
// one of the map's own has is replaced by it.
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

// labelOf is how errors name a library's folder, and how a report names a model a command line names: by its
// path from the project folder with "/" where it is inside the project, else by its path.
func labelOf(root, dir string) string {
	if below, err := filepath.Rel(root, dir); err == nil && filepath.IsLocal(below) {
		return filepath.ToSlash(below)
	}
	return filepath.ToSlash(dir)
}

// refuseLinked fails when a link stands in place of a library's folder, which errors name by label. A link is made
// on this machine and is not among what the library ships, so the failure is not the library's.
func refuseLinked(dir, label string) error {
	if info, err := fsx.Lstat(dir); err == nil && info != nil && fsx.IsLink(info) {
		return errLinkedFolder(dir, label)
	}
	return nil
}

// addShipped adds one file of a library, unless one of the map's own is imported at its path. A path the file may
// not have is the library's failure; two libraries at one path are the project's.
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

// replace notes that the map's own asset stands in for a library's file at the same in-map path. An asset of
// another library cannot: the two libraries are refused.
func (c *collection) replace(other Asset, library, source, target string) error {
	if other.Library != "" {
		return errTwoLibraries(c.manifestFile, other.Library, library, target)
	}
	c.replaced = append(c.replaced, "assets/"+other.Source+" replaces library "+library+"'s "+source)
	return nil
}

// inLibrary turns a failure about what a library's folder contains into the library's, which the map's author can
// only report: a path no asset may have, and what a folder of map files cannot hold. A failure with a cause is
// the system's (a file or a folder that cannot be read) and stays as it is, at the file it names with its own
// hint. So do nil and an error that is not an expected failure.
func inLibrary(err error, library, label string) error {
	var failure *diag.Error
	if !errors.As(err, &failure) || failure.Cause != nil {
		return err
	}
	return errInLibrary(library, label, failure)
}

// ---- folders and files ----

// open scans a folder of files for the map, which errors name by label. A folder that is not there is nil, and
// has no files. What a folder cannot hold (a link, two spellings of one path, a name Windows cannot hold) is
// refused as it is for a map folder: these files become map files.
func open(dir, label string) (*mapdir.Folder, error) {
	if info, err := fsx.Lstat(dir); err == nil && info != nil && !info.IsDir() && !fsx.IsLink(info) {
		return nil, errNotAFolder(label)
	}
	folder, err := mapdir.Open(dir, label)
	// Only the folder itself may be missing. A folder below it that is gone by the time it is listed is an
	// expected failure, which names that folder.
	if _, expected := diag.First(err); !expected && errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	return folder, err
}

// filesOf is the files of a folder as it spells them, in the order of its scan.
func filesOf(folder *mapdir.Folder) []string {
	if folder == nil {
		return nil
	}
	return folder.Files()
}

// add reads the asset's file from its folder and takes the asset into the collection.
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

// refuseNesting fails when the in-map path of one asset is a folder on the way to another's: a map cannot hold
// a file and a folder under one name.
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

// sortByTarget puts the assets in the order of their in-map paths, compared byte by byte without regard to
// letter case.
func (c *collection) sortByTarget() {
	slices.SortStableFunc(c.assets, func(a, b Asset) int {
		return strings.Compare(mapdir.Key(a.Target), mapdir.Key(b.Target))
	})
}

// ---- errors ----

// errInvalidPath, for a source or an exclusion that is no path, is with targetPath in target.go.

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

// errLinkedFolder is fsx.LinkError for the link at dir, with the folder as its file: label, the name the folder
// has in every other error about it.
func errLinkedFolder(dir, label string) error {
	err := fsx.LinkError(dir)
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
