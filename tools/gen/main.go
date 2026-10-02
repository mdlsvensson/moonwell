// Command gen writes Moonwell's generated files. Run it from the checkout:
//
//	go run ./tools/gen                                  schema/generated/*.pkl, from the object metadata
//	go run ./tools/gen natives <folder> <version>       the natives, from common.j and blizzard.j
//	go run ./tools/gen metadata <folder> <version>      the object metadata, from the game's SLK and text files
//	go run ./tools/gen game-paths <listfile> <version>  the in-game path list, from a CASC file-name export
//
// The folders are exports made with CascView, which keeps the game's relative paths (war3.w3mod/...).
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/objects"
	"github.com/mdlsvensson/moonwell/internal/text"
	"github.com/mdlsvensson/moonwell/tools/gen/jass"
)

const (
	// generatedBy and metadataPath go into the first line of each generated schema file.
	generatedBy  = "go run ./tools/gen"
	metadataPath = "data/metadata.json"

	nativesPath   = "data/natives.json"
	gamePathsPath = "data/game-paths.txt"
	overridesPath = "tools/metadata/overrides.json"
	extrasPath    = "tools/natives/lua-extras.json"
	schemaFolder  = "schema/generated"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error: "+err.Error())
		os.Exit(1)
	}
}

func run(args []string) error {
	root, err := checkout()
	if err != nil {
		return err
	}
	file := func(path string) string { return filepath.Join(root, filepath.FromSlash(path)) }
	mode := ""
	if len(args) > 0 {
		mode, args = args[0], args[1:]
	}
	switch mode {
	case "":
		return writeSchema(root)
	case "natives":
		if len(args) != 2 {
			return errors.New("Usage: go run ./tools/gen natives <exported folder> <game version>")
		}
		return writeNatives(args[0], args[1], file(extrasPath), file(nativesPath))
	case "metadata":
		if len(args) != 2 {
			return errors.New("Usage: go run ./tools/gen metadata <game data folder> <game version, e.g. 3.0.0.24268>")
		}
		return writeMetadata(args[0], args[1], file(overridesPath), file(metadataPath))
	case "game-paths":
		if len(args) != 2 {
			return errors.New("Usage: go run ./tools/gen game-paths <listfile> <game version, e.g. 3.0.0.24268>")
		}
		list, err := os.ReadFile(args[0])
		if err != nil {
			return err
		}
		count, err := GenerateGamePaths(text.Lossy(list), args[1], file(gamePathsPath))
		if err != nil {
			return err
		}
		fmt.Println("wrote " + gamePathsPath + ": " + strconv.Itoa(count) + " paths.")
		return nil
	}
	return errors.New("Unknown mode '" + mode + "'. The modes are natives, metadata and game-paths; without one, gen " +
		"writes " + schemaFolder + ".")
}

var moduleLine = regexp.MustCompile(`(?m)^module\s+github\.com/mdlsvensson/moonwell\s*$`)

// checkout finds the Moonwell checkout: the folder at or above the working directory whose go.mod names this
// module.
func checkout() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for ; ; dir = filepath.Dir(dir) {
		if data, err := os.ReadFile(filepath.Join(dir, "go.mod")); err == nil && moduleLine.Match(data) {
			return dir, nil
		}
		if dir == filepath.Dir(dir) {
			return "", errors.New("run gen in a Moonwell checkout")
		}
	}
}

// readMetadata reads the object metadata of the checkout at root.
func readMetadata(root string) (*objects.Metadata, error) {
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(metadataPath)))
	if err != nil {
		return nil, err
	}
	var metadata objects.Metadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, errors.New(metadataPath + ": " + err.Error())
	}
	return &metadata, nil
}

// writeSchema writes schema/generated/ from the object metadata, and removes what it did not write: the folder
// holds nothing else.
func writeSchema(root string) error {
	metadata, err := readMetadata(root)
	if err != nil {
		return err
	}
	files, err := RenderSchema(metadata)
	if err != nil {
		return err
	}
	written := map[string]bool{}
	for _, file := range files {
		written[file.Path] = true
		changed, err := fsx.WriteIfChanged(filepath.Join(root, filepath.FromSlash(file.Path)), file.Text)
		if err != nil {
			return err
		}
		if changed {
			fmt.Println("wrote " + file.Path)
		}
	}
	folder := filepath.Join(root, filepath.FromSlash(schemaFolder))
	entries, err := os.ReadDir(folder)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		path := schemaFolder + "/" + entry.Name()
		if written[path] {
			continue
		}
		if err := os.RemoveAll(filepath.Join(folder, entry.Name())); err != nil {
			return err
		}
		fmt.Println("removed " + path)
	}
	return nil
}

func writeNatives(folder, version, extrasFile, target string) error {
	read := func(path string) (jass.File, error) {
		data, err := os.ReadFile(filepath.Join(folder, filepath.FromSlash(path)))
		if err != nil {
			return jass.File{}, err
		}
		// Entries record the lower-case file name ("common.j", "blizzard.j") whatever the export's letter case.
		return jass.Parse(text.Lossy(data), text.Lower(path[strings.LastIndexByte(path, '/')+1:]))
	}
	common, err := read(commonFile)
	if err != nil {
		return err
	}
	blizzard, err := read(blizzardFile)
	if err != nil {
		return err
	}
	extras, err := os.ReadFile(extrasFile)
	if err != nil {
		return err
	}
	natives, err := BuildNatives(version, common, blizzard, extras)
	if err != nil {
		return err
	}
	if err := os.WriteFile(target, []byte(RenderNatives(natives)), 0o666); err != nil {
		return err
	}
	count := func(key string) string {
		list, _ := natives.Get(key)
		return strconv.Itoa(len(list.([]any)))
	}
	fmt.Println("wrote " + nativesPath + ": " + count("types") + " types, " + count("functions") + " functions, " +
		count("globals") + " globals")
	return nil
}

func writeMetadata(folder, version, overridesFile, target string) error {
	data, err := os.ReadFile(overridesFile)
	if err != nil {
		return err
	}
	var overrides Overrides
	if err := json.Unmarshal(data, &overrides); err != nil {
		return errors.New(overridesPath + ": " + err.Error())
	}
	result, err := GenerateMetadata(folder, version, target, overrides)
	if err != nil {
		return err
	}
	counts := func(list []Count) string {
		parts := make([]string, len(list))
		for i, entry := range list {
			parts[i] = text.Quote(entry.Category) + ":" + strconv.Itoa(entry.Count)
		}
		return "{" + strings.Join(parts, ",") + "}"
	}
	fmt.Println("fields: " + counts(result.Fields))
	fmt.Println("bases: " + counts(result.Bases))
	fmt.Println("renamed (" + strconv.Itoa(len(result.Renames)) + "):")
	for _, rename := range result.Renames {
		fmt.Println("  " + rename)
	}
	fmt.Println("wrote " + metadataPath + ". Now run `go run ./tools/gen`.")
	return nil
}
