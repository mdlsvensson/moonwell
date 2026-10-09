package assets

import (
	"errors"
	"path/filepath"
	"slices"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/manifest"
	"github.com/mdlsvensson/moonwell/internal/mapdir"
)

const projectAssetsDir = "assets"

func (c *collector) addProjectAssets(root string, config manifest.Assets) error {
	folder, err := openDir(filepath.Join(root, projectAssetsDir), projectAssetsDir)
	if err != nil {
		return err
	}
	rules, err := parseExcludeRules(config.Exclude)
	if err != nil {
		return c.blameAssetsBlock(err)
	}
	files := sortedFiles(folder)
	targets, err := c.resolveMappings(config.Paths, files, rules)
	if err != nil {
		return err
	}
	for _, source := range files {
		if isExcluded(source, rules) {
			continue
		}
		if err := c.addProjectFile(folder, source, targets); err != nil {
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
		name, isDir := value, false
		if strings.HasSuffix(value, "/") || strings.HasSuffix(value, `\`) {
			name, isDir = value[:len(value)-1], true
		}
		path, ok := fsx.CleanRelPath(name)
		if !ok {
			return nil, errInvalidPath(name)
		}
		rules = append(rules, excludeRule{mapdir.Key(path), isDir})
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
	exists := map[string]bool{}
	for _, file := range files {
		exists[mapdir.Key(file)] = true
	}
	targets := map[string]string{}
	for source, target := range paths.All() {
		path, ok := fsx.CleanRelPath(source)
		if !ok {
			return nil, c.blameAssetsBlock(errInvalidPath(source))
		}
		key := mapdir.Key(path)
		_, isDuplicate := targets[key]
		switch {
		case !exists[key]:
			return nil, errNoSuchFile(c.manifestName, source)
		case isExcluded(path, rules):
			return nil, errExcluded(c.manifestName, source)
		case isDuplicate:
			return nil, errNamedTwice(c.manifestName, source)
		}
		var err error
		if targets[key], err = parseTargetPath(target); err != nil {
			return nil, c.blameAssetsBlock(err)
		}
	}
	return targets, nil
}

func (c *collector) addProjectFile(folder *mapdir.Folder, source string, targets map[string]string) error {
	target, isMapped := targets[mapdir.Key(source)]
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

func (c *collector) blameAssetsBlock(err error) error {
	var diagErr *diag.Error
	if errors.As(err, &diagErr) {
		diagErr.File = c.manifestName
		diagErr.Hint += " " + blockHint
	}
	return err
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
