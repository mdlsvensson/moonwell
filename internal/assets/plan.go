package assets

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/mapdir"
	"github.com/mdlsvensson/moonwell/internal/ordered"
)

// State is .asset-state/<map folder>.json: the source-map files assets:sync owns, by in-map path, with their
// SHA-256.
type State struct {
	Version int
	Files   ordered.Map[string]
}

// FileChange is one file to write or to delete.
type FileChange struct {
	// File is the absolute path.
	File string
	// Before is the file's content at planning time; Existed says whether it was there at all.
	Before  []byte
	Existed bool
	// After is the content to write; Remove deletes the file instead.
	After  []byte
	Remove bool
}

// Plan is what importing the assets changes in a map folder.
type Plan struct {
	Assets []*Asset
	// Replaced has a line for each library file that one of the map's own assets replaces.
	Replaced []string
	Changes  []FileChange
	// State is the ownership after applying the plan.
	State State
}

// Locations returns the source map folder and its ownership state file.
func Locations(root, mapFolder string) (mapDir, stateFile string, err error) {
	if mapDir, err = fsx.SafeJoin(root, "maps/"+mapFolder); err != nil {
		return "", "", err
	}
	stateFile, err = fsx.SafeJoin(root, ".asset-state/"+mapFolder+".json")
	return mapDir, stateFile, err
}

// readIfExists reads a file; exists is false, without an error, when there is none.
func readIfExists(file string) (data []byte, exists bool, err error) {
	info, err := fsx.Lstat(file)
	if err != nil || info == nil {
		return nil, false, err
	}
	data, err = os.ReadFile(file)
	return data, err == nil, err
}

var sha256Hex = regexp.MustCompile(`^[a-f0-9]{64}$`)

const stateHint = "Restore it from version control. It records which map files assets:sync owns."

func readState(file string) (*ordered.Map[string], error) {
	files := &ordered.Map[string]{}
	data, exists, err := readIfExists(file)
	if err != nil || !exists {
		return files, err
	}
	invalid := func(problem string) (*ordered.Map[string], error) {
		return nil, &diag.Error{Msg: "The asset ownership state is invalid: " + problem + ".", File: file, Hint: stateHint}
	}
	tree, err := ordered.Decode(data)
	if err != nil {
		return invalid("it is not JSON")
	}
	record, _ := tree.(*ordered.Object)
	if record == nil {
		return invalid("version must be 1")
	}
	if version, _ := record.Get("version"); version != 1.0 {
		return invalid("version must be 1")
	}
	listed, _ := record.Get("files")
	entries, ok := listed.(*ordered.Object)
	if !ok {
		return invalid("files must be an object")
	}
	seen := map[string]bool{}
	for name, digest := range entries.All() {
		if _, err := TargetPath(name); err != nil {
			var failure *diag.Error
			if !errors.As(err, &failure) {
				return nil, err
			}
			return nil, &diag.Error{
				Msg: "The asset ownership state is invalid: " + failure.Msg, File: file, Hint: stateHint, Cause: failure,
			}
		}
		hash, isString := digest.(string)
		if !isString || !sha256Hex.MatchString(hash) {
			return invalid(name + " has no valid hash")
		}
		if seen[mapdir.Key(name)] {
			return invalid(name + " is listed twice")
		}
		seen[mapdir.Key(name)] = true
		files.Set(name, hash)
	}
	return files, nil
}

const interruptedBeforeWriting = "Interrupted; nothing was written."

// PlanAssets checks every asset (the map's own and those of libraries, by key), import and owned file, and returns
// the changes. It writes nothing, and stops with an error between files once ctx is cancelled (Ctrl+C).
func PlanAssets(ctx context.Context, root, mapDir, stateFile string, config Config, libraries []string) (*Plan, error) {
	stopIfInterrupted := func() error {
		if ctx.Err() != nil {
			return &diag.Error{Msg: interruptedBeforeWriting}
		}
		return nil
	}
	assets, replaced, err := CollectProject(root, config, libraries)
	if err != nil {
		return nil, err
	}
	if err := stopIfInterrupted(); err != nil {
		return nil, err
	}
	if info, err := fsx.Lstat(mapDir); err != nil {
		return nil, err
	} else if info == nil || !info.IsDir() {
		return nil, &diag.Error{
			Msg:  "The map folder " + mapDir + " does not exist.",
			File: "moonwell.pkl",
			Hint: "Set map.folder to a folder under maps/ saved by World Editor in folder format.",
		}
	}
	owned, err := readState(stateFile)
	if err != nil {
		return nil, err
	}
	type ownedFile struct{ name, digest string }
	managed := map[string]ownedFile{}
	var managedKeys []string
	for name, digest := range owned.All() {
		key := mapdir.Key(name)
		if _, again := managed[key]; !again {
			managedKeys = append(managedKeys, key)
		}
		managed[key] = ownedFile{name, digest}
	}
	plan := &Plan{Assets: assets, Replaced: replaced, Changes: []FileChange{}, State: State{Version: 1}}
	// Nothing to import and nothing owned: leave the map (and its war3map.imp) completely alone.
	if len(assets) == 0 && len(managed) == 0 {
		return plan, nil
	}
	files, err := ScanFiles(mapDir)
	if err != nil {
		return nil, err
	}
	impName, ok := files.Get("war3map.imp")
	if !ok {
		impName = "war3map.imp"
	}
	impFile, err := fsx.SafeJoin(mapDir, impName)
	if err != nil {
		return nil, err
	}
	impBytes, impExists, err := readIfExists(impFile)
	if err != nil {
		return nil, err
	}
	var imports []Import
	if impExists {
		if imports, err = ReadImports(impBytes, impFile); err != nil {
			return nil, err
		}
	}
	importKeys := map[string]bool{}
	for _, entry := range imports {
		path, err := fsx.RelPath(ImportPath(entry))
		if err != nil {
			return nil, err
		}
		if key := mapdir.Key(path); importKeys[key] {
			return nil, &diag.Error{
				Msg:  "war3map.imp lists " + ImportPath(entry) + " twice.",
				File: impFile,
				Hint: "Remove the duplicate import in World Editor's Import Manager.",
			}
		} else {
			importKeys[key] = true
		}
	}

	// Every owned file must be unchanged, including files this plan would delete.
	for _, key := range managedKeys {
		if err := stopIfInterrupted(); err != nil {
			return nil, err
		}
		current, ok := files.Get(key)
		if !ok {
			continue
		}
		file, err := fsx.SafeJoin(mapDir, current)
		if err != nil {
			return nil, err
		}
		data, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}
		if fsx.SHA256Hex(data) != managed[key].digest {
			return nil, &diag.Error{
				Msg:  current + " was modified in the map after assets:sync wrote it.",
				File: file,
				Hint: "assets:sync owns this file in the source map (a build stages a copy of it). " +
					"Move your edited copy into assets/, or restore the file in the source map, then run assets:sync.",
			}
		}
	}

	plannedFolders := map[string]string{}
	for _, asset := range assets {
		if err := stopIfInterrupted(); err != nil {
			return nil, err
		}
		key := mapdir.Key(asset.Target)
		if _, isOwned := managed[key]; !isOwned && (files.Has(key) || importKeys[key]) {
			owner, hint := "", "Import it under another path with assets.paths, or remove the map's own copy."
			if asset.Library != "" {
				owner, hint = " of library "+asset.Library, "Remove the map's own copy in World Editor's Import Manager."
			}
			return nil, &diag.Error{
				Msg:  "Asset " + asset.Target + owner + " conflicts with a file or import already in the map.",
				Hint: hint,
			}
		}
		// Reuse the spelling of existing folders, and of folders planned earlier, so letter case stays consistent.
		parts := strings.Split(asset.Target, "/")
		relative := ""
		for i, part := range parts {
			parent := mapDir
			if relative != "" {
				if parent, err = fsx.SafeJoin(mapDir, relative); err != nil {
					return nil, err
				}
			}
			info, err := fsx.Lstat(parent)
			if err != nil {
				return nil, err
			}
			if info != nil && !info.IsDir() {
				return nil, &diag.Error{
					Msg: relative + " in the map is a file, not a directory, so " + asset.Target + " cannot go there.",
				}
			}
			existing := ""
			if info != nil {
				entries, err := os.ReadDir(parent)
				if err != nil {
					return nil, err
				}
				for _, entry := range entries {
					if mapdir.Key(entry.Name()) != mapdir.Key(part) {
						continue
					}
					if entryInfo, err := entry.Info(); err == nil && fsx.IsLink(entryInfo) {
						return nil, &diag.Error{Msg: "Symlinks are not supported: " + filepath.Join(parent, entry.Name())}
					}
					existing = entry.Name()
					break
				}
			}
			folderKey := mapdir.Key(part)
			if relative != "" {
				folderKey = mapdir.Key(relative + "/" + part)
			}
			segment := part
			if existing != "" {
				segment = existing
			} else if planned, ok := plannedFolders[folderKey]; ok {
				segment = planned
			}
			if relative == "" {
				relative = segment
			} else {
				relative += "/" + segment
			}
			if i < len(parts)-1 {
				plannedFolders[folderKey] = segment
			}
		}
		file, err := fsx.SafeJoin(mapDir, relative)
		if err != nil {
			return nil, err
		}
		if info, err := fsx.Lstat(file); err != nil {
			return nil, err
		} else if info != nil && info.IsDir() {
			return nil, &diag.Error{Msg: "Asset " + asset.Target + " would replace a folder in the map."}
		}
		before, existed, err := readIfExists(file)
		if err != nil {
			return nil, err
		}
		if !existed || fsx.SHA256Hex(before) != asset.Hash {
			plan.Changes = append(plan.Changes, FileChange{File: file, Before: before, Existed: existed, After: asset.Bytes})
		}
		asset.Target = relative
	}

	wanted := map[string]bool{}
	for _, asset := range assets {
		wanted[mapdir.Key(asset.Target)] = true
	}
	for _, key := range managedKeys {
		current, ok := files.Get(key)
		if !ok || wanted[key] {
			continue
		}
		file, err := fsx.SafeJoin(mapDir, current)
		if err != nil {
			return nil, err
		}
		before, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}
		plan.Changes = append(plan.Changes, FileChange{File: file, Before: before, Existed: true, Remove: true})
	}

	// An owned entry keeps the flag World Editor saved it with (3.00 turns 13 into 29), so a save changes nothing.
	ownedFlags := map[string]uint8{}
	merged := []Import{}
	for _, entry := range imports {
		key := mapdir.Key(ImportPath(entry))
		if _, isOwned := managed[key]; isOwned {
			ownedFlags[key] = entry.Flag
		} else {
			merged = append(merged, entry)
		}
	}
	for _, asset := range assets {
		flag, kept := ownedFlags[mapdir.Key(asset.Target)]
		if !kept {
			flag = 13
		}
		merged = append(merged, Import{Flag: flag, Path: strings.ReplaceAll(asset.Target, "/", `\`)})
	}
	if after := WriteImports(merged); !impExists || !bytes.Equal(after, impBytes) {
		plan.Changes = append(plan.Changes, FileChange{File: impFile, Before: impBytes, Existed: impExists, After: after})
	}
	for _, asset := range assets {
		plan.State.Files.Set(asset.Target, asset.Hash)
	}
	return plan, nil
}

// stateBytes is the content of the state file for a plan.
func stateBytes(state State) []byte {
	document := &ordered.Map[any]{}
	document.Set("version", state.Version)
	document.Set("files", &state.Files)
	return []byte(ordered.Stringify(document, 2) + "\n")
}

// ApplyPlan applies a plan, undoing every change already made if one fails or ctx is cancelled (Ctrl+C). With a
// stateFile it writes the ownership state too; a build, which only changes the staged copy, passes "".
func ApplyPlan(ctx context.Context, plan *Plan, stateFile string) error {
	changes := append([]FileChange{}, plan.Changes...)
	if stateFile != "" {
		before, existed, err := readIfExists(stateFile)
		if err != nil {
			return err
		}
		// Owning nothing needs no state file, so an old one is removed.
		change := FileChange{File: stateFile, Before: before, Existed: existed, Remove: plan.State.Files.Len() == 0}
		if !change.Remove {
			change.After = stateBytes(plan.State)
		}
		if unchanged := existed && !change.Remove && bytes.Equal(before, change.After); !unchanged && (existed || !change.Remove) {
			changes = append(changes, change)
		}
	}
	var applied []FileChange
	interrupted := errors.New("interrupted")
	failure := func() error {
		for _, change := range changes {
			if ctx.Err() != nil {
				return interrupted
			}
			current, exists, err := readIfExists(change.File)
			if err != nil {
				return err
			}
			if exists != change.Existed || (exists && !bytes.Equal(current, change.Before)) {
				return &diag.Error{
					Msg:  change.File + " changed after the assets were checked.",
					Hint: "Close World Editor and anything else writing to the map, then retry.",
				}
			}
			applied = append(applied, change)
			if change.Remove {
				if err := os.Remove(change.File); err != nil {
					return err
				}
				continue
			}
			if err := os.MkdirAll(filepath.Dir(change.File), 0o777); err != nil {
				return err
			}
			if err := os.WriteFile(change.File, change.After, 0o666); err != nil {
				return err
			}
		}
		return nil
	}()
	if failure == nil {
		return nil
	}

	reasonOf := func(err error) string {
		var userError *diag.Error
		if errors.As(err, &userError) {
			return userError.Msg
		}
		return fsx.Reason(err)
	}
	var unrestored []string
	for i := len(applied) - 1; i >= 0; i-- {
		change := applied[i]
		var err error
		if change.Existed {
			err = os.WriteFile(change.File, change.Before, 0o666)
		} else {
			err = fsx.RemoveFile(change.File)
		}
		if err != nil {
			unrestored = append(unrestored, change.File+" ("+reasonOf(err)+")")
		}
	}
	var userError *diag.Error
	switch {
	case len(unrestored) > 0:
		return &diag.Error{
			Msg: "Writing assets failed (" + reasonOf(failure) + "), and these files could not be restored: " +
				strings.Join(unrestored, ", "),
			Cause: failure,
			Hint:  "Restore the map folder from version control before retrying.",
		}
	case errors.As(failure, &userError):
		return failure
	case failure == interrupted && len(applied) == 0:
		return &diag.Error{Msg: interruptedBeforeWriting}
	case failure == interrupted:
		return &diag.Error{Msg: "Interrupted; every change was undone."}
	}
	return &diag.Error{Msg: "Writing assets failed: " + reasonOf(failure) + ". Every change was undone.", Cause: failure}
}
