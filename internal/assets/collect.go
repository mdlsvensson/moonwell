package assets

import (
	"errors"
	"os"
	"slices"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/layout"
	"github.com/mdlsvensson/moonwell/internal/mapdir"
	"github.com/mdlsvensson/moonwell/internal/ordered"
	"github.com/mdlsvensson/moonwell/internal/text"
)

// Config is the manifest's assets block: Paths maps assets/ files to exact in-map paths; Exclude leaves files
// out.
type Config struct {
	Paths   *ordered.Map[string]
	Exclude []string
}

// Asset is a file under assets/, or one a library ships, and the in-map path it is imported as.
type Asset struct {
	// Source is the path under assets/, or under the library's assets folder, with "/".
	Source string
	// Library is the key of the library that ships the file; empty for the map's own.
	Library string
	// Target is the in-map path, with "/".
	Target string
	Bytes  []byte
	// Hash is the SHA-256 of Bytes, in lower-case hexadecimal.
	Hash string
}

func configError(message string) error {
	return &diag.Error{Msg: message, File: "moonwell.pkl", Hint: "Fix the assets block in moonwell.pkl."}
}

func hasDotPart(path string) bool {
	return slices.ContainsFunc(strings.Split(path, "/"), func(part string) bool { return strings.HasPrefix(part, ".") })
}

func sortByTarget(assets []*Asset) {
	slices.SortStableFunc(assets, func(a, b *Asset) int {
		return text.Compare(mapdir.Key(a.Target), mapdir.Key(b.Target))
	})
}

// Collect reads assets/ under root and resolves every file's in-map target. Nothing is written.
func Collect(root string, config Config) ([]*Asset, error) {
	folder, err := fsx.SafeJoin(root, "assets")
	if err != nil {
		return nil, err
	}
	files, err := ScanFiles(folder)
	if err != nil {
		return nil, err
	}
	type exclusion struct {
		folder bool
		key    string
	}
	var exclusions []exclusion
	for _, value := range config.Exclude {
		isFolder := strings.HasSuffix(value, "/") || strings.HasSuffix(value, `\`)
		if isFolder {
			value = value[:len(value)-1]
		}
		path, err := fsx.RelPath(value)
		if err != nil {
			return nil, err
		}
		exclusions = append(exclusions, exclusion{isFolder, mapdir.Key(path)})
	}
	excluded := func(key string) bool {
		return hasDotPart(key) || slices.ContainsFunc(exclusions, func(rule exclusion) bool {
			return key == rule.key || (rule.folder && strings.HasPrefix(key, rule.key+"/"))
		})
	}

	mappings := map[string]string{}
	if config.Paths != nil {
		for source, target := range config.Paths.All() {
			path, err := fsx.RelPath(source)
			if err != nil {
				return nil, err
			}
			key := mapdir.Key(path)
			if !files.Has(key) {
				return nil, configError("assets.paths names a file that does not exist: assets/" + source)
			}
			if excluded(key) {
				return nil, configError("assets.paths names an excluded file: " + source)
			}
			if _, twice := mappings[key]; twice {
				return nil, configError("assets.paths names " + source + " twice.")
			}
			if mappings[key], err = TargetPath(target); err != nil {
				return nil, err
			}
		}
	}

	var targets []string
	assets := []*Asset{}
	for _, key := range files.Keys() {
		if excluded(key) {
			continue
		}
		source, _ := files.Get(key)
		target, mapped := mappings[key]
		if !mapped {
			if target, err = TargetPath(source); err != nil {
				return nil, err
			}
		}
		if slices.Contains(targets, mapdir.Key(target)) {
			return nil, configError("Two assets would be imported as " + target + " (target collision).")
		}
		targets = append(targets, mapdir.Key(target))
		asset, err := readAsset(folder, source, target)
		if err != nil {
			return nil, err
		}
		assets = append(assets, asset)
	}
	if err := refuseNesting(targets); err != nil {
		return nil, err
	}
	sortByTarget(assets)
	return assets, nil
}

func readAsset(folder, source, target string) (*Asset, error) {
	path, err := fsx.SafeJoin(folder, source)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return &Asset{Source: source, Target: target, Bytes: data, Hash: fsx.SHA256Hex(data)}, nil
}

// refuseNesting fails when one target would be a folder that another target is a file in. targets holds path
// keys.
func refuseNesting(targets []string) error {
	for _, target := range targets {
		parts := strings.Split(target, "/")
		for len(parts) > 1 {
			parts = parts[:len(parts)-1]
			if parent := strings.Join(parts, "/"); slices.Contains(targets, parent) {
				return configError("Asset " + target + " would sit inside the asset file " + parent + " (file/folder collision).")
			}
		}
	}
	return nil
}

// CollectProject returns the map's own assets and then the files its libraries ship
// (.moonwell/library-assets/<key>/, as the last library sync left them), in key order. A library file's in-map
// path is its path in that folder. The map's own file wins over a library's at the same in-map path, with a line
// in replaced saying so; two libraries at one path fail. Nothing is written.
func CollectProject(root string, config Config, libraries []string) (assets []*Asset, replaced []string, err error) {
	if assets, err = Collect(root, config); err != nil {
		return nil, nil, err
	}
	taken := map[string]*Asset{}
	var takenKeys []string
	for _, asset := range assets {
		taken[mapdir.Key(asset.Target)] = asset
		takenKeys = append(takenKeys, mapdir.Key(asset.Target))
	}
	replaced = []string{}
	sorted := slices.Clone(libraries)
	text.Sort(sorted)
	for _, key := range sorted {
		label := layout.LibraryAssetsDir + "/" + key
		folder, err := fsx.SafeJoin(root, label)
		if err != nil {
			return nil, nil, err
		}
		// fromLibrary turns a user-facing failure into the library's, which the map's author can only report.
		fromLibrary := func(err error) error {
			var failure *diag.Error
			if !errors.As(err, &failure) {
				return err
			}
			return &diag.Error{
				Msg:   "Library " + key + ": " + failure.Msg,
				File:  label,
				Hint:  "Report it to the library's author, or use another version of the library.",
				Cause: failure,
			}
		}
		files, err := ScanFiles(folder)
		if err != nil {
			return nil, nil, fromLibrary(err)
		}
		for _, fileKey := range files.Keys() {
			source, _ := files.Get(fileKey)
			if hasDotPart(source) {
				continue
			}
			target, err := TargetPath(source)
			if err != nil {
				return nil, nil, fromLibrary(err)
			}
			if other, ok := taken[mapdir.Key(target)]; ok {
				if other.Library == "" {
					replaced = append(replaced, "assets/"+other.Source+" replaces library "+key+"'s "+source)
					continue
				}
				return nil, nil, &diag.Error{
					Msg:  "Libraries " + other.Library + " and " + key + " both import " + strings.ReplaceAll(target, "/", `\`) + ".",
					File: "moonwell.pkl",
					Hint: "Drop one of the libraries, or put your own file at that path under assets/ to replace both.",
				}
			}
			asset, err := readAsset(folder, source, target)
			if err != nil {
				return nil, nil, err
			}
			asset.Library = key
			assets = append(assets, asset)
			taken[mapdir.Key(target)] = asset
			takenKeys = append(takenKeys, mapdir.Key(target))
		}
	}
	if err := refuseNesting(takenKeys); err != nil {
		return nil, nil, err
	}
	sortByTarget(assets)
	return assets, replaced, nil
}
