package cli

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/assets"
	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/mapdir"
	"github.com/mdlsvensson/moonwell/internal/models"
	"github.com/mdlsvensson/moonwell/internal/pipeline"
	"github.com/mdlsvensson/moonwell/internal/project"
	"github.com/mdlsvensson/moonwell/internal/text"
)

// PathStatus says where a file a model references comes from.
type PathStatus string

const (
	InGame            PathStatus = "in-game path"
	InGameReplaced    PathStatus = "in-game path, replaced"
	CustomImported    PathStatus = "custom path, imported"
	CustomNotImported PathStatus = "custom path, not imported"
	Custom            PathStatus = "custom path"
)

// ModelRef is one file a model references. Status is "" for a reference without a path (a team colour slot).
type ModelRef struct {
	models.Path
	Status PathStatus
}

// ModelReport is the references of one model.
type ModelReport struct {
	Heading string
	Refs    []ModelRef
}

var (
	mdlExtension = regexp.MustCompile(`\.mdl$`)
	modelFile    = regexp.MustCompile(`(?i)\.(mdx|mdl)$`)
)

// referenceKey is how a reference is compared with the import targets (by mapdir.Key): any letter case, either
// separator, and a requested .mdl as the .mdx, which the game loads instead. An imported .mdl is never loaded, so
// targets keep theirs.
func referenceKey(path string) string {
	return mdlExtension.ReplaceAllString(mapdir.Key(path), ".mdx")
}

func plural(count int, word string) string {
	if count == 1 {
		return "1 " + word
	}
	return strconv.Itoa(count) + " " + word + "s"
}

// pathStatus is a reference's status: whether the game ships its path and, in a project (targets is not nil),
// whether a build imports it.
func pathStatus(path string, gamePaths, targets map[string]bool) PathStatus {
	inGame := gamePaths[models.GamePathKey(path)]
	if targets == nil {
		if inGame {
			return InGame
		}
		return Custom
	}
	imported := targets[referenceKey(path)]
	switch {
	case inGame && imported:
		return InGameReplaced
	case inGame:
		return InGame
	case imported:
		return CustomImported
	}
	return CustomNotImported
}

// model is a model file to report on.
type model struct {
	heading string
	data    []byte
}

// AssetsPaths lists the files a model references (the model at file, or every model under assets/ and among the
// files the libraries ship when file is "") as in-game or custom paths. In a project it syncs the libraries first:
// the files they ship count as imported. A nil gamePaths is the embedded list.
func AssetsPaths(ctx context.Context, env *pipeline.Env, file string, gamePaths map[string]bool) ([]ModelReport, error) {
	inProject := fsx.Exists(filepath.Join(env.Root, "moonwell.pkl"))
	var imported []*assets.Asset
	var targets map[string]bool
	if inProject {
		p, err := project.Load(ctx, env.Root, env.Run)
		if err != nil {
			return nil, err
		}
		if err := pipeline.SyncLibraries(ctx, env, p); err != nil {
			return nil, err
		}
		config := assets.Config{Paths: p.Assets.Paths, Exclude: p.Assets.Exclude}
		if imported, _, err = assets.CollectProject(env.Root, config, p.LibraryKeys()); err != nil {
			return nil, err
		}
		targets = map[string]bool{}
		for _, asset := range imported {
			targets[mapdir.Key(asset.Target)] = true
		}
	}

	var found []model
	switch {
	case file != "":
		one, err := readModel(env.Root, file)
		if err != nil {
			return nil, err
		}
		found = []model{one}
	case !inProject:
		return nil, &diag.Error{
			Msg:  "assets:paths needs a model file outside a Moonwell project.",
			Hint: "moonwell assets:paths assets/Models/Knight.mdx",
		}
	default:
		for _, asset := range imported {
			if !modelFile.MatchString(asset.Target) {
				continue
			}
			heading := "assets/" + asset.Source
			if asset.Library != "" {
				heading = "library " + asset.Library + ": " + asset.Source
			}
			found = append(found, model{heading, asset.Bytes})
		}
		if len(found) == 0 {
			env.Log.Info("No models under assets/.")
			return []ModelReport{}, nil
		}
	}

	if gamePaths == nil {
		gamePaths = models.LoadGamePaths()
	}
	if len(gamePaths) == 0 {
		env.Log.Warn("Moonwell's in-game path list is empty, so every path shows as custom.")
	}
	// With no file given, an unreadable model is reported in its place instead of hiding every other model.
	type entry struct {
		report  ModelReport
		failure string
	}
	var reports []ModelReport
	var unreadable []string
	var entries []entry
	for _, model := range found {
		refs, err := models.Paths(model.data, model.heading)
		if err != nil {
			var failure *diag.Error
			if file != "" || !errors.As(err, &failure) {
				return nil, err
			}
			unreadable = append(unreadable, model.heading)
			entries = append(entries, entry{report: ModelReport{Heading: model.heading}, failure: failure.Msg})
			continue
		}
		report := ModelReport{Heading: model.heading, Refs: make([]ModelRef, len(refs))}
		for i, ref := range refs {
			report.Refs[i] = ModelRef{Path: ref}
			if ref.Path != "" {
				report.Refs[i].Status = pathStatus(ref.Path, gamePaths, targets)
			}
		}
		reports = append(reports, report)
		entries = append(entries, entry{report: report})
	}

	label := func(ref ModelRef) string { return strings.ReplaceAll(models.Describe(ref.Path), "/", `\`) }
	statuses := map[PathStatus]int{}
	paths := 0
	for _, entry := range entries {
		env.Log.Info(entry.report.Heading)
		if entry.failure != "" {
			env.Log.Info("  (unreadable: " + entry.failure + ")")
			continue
		}
		if len(entry.report.Refs) == 0 {
			env.Log.Info("  (no referenced files)")
		}
		kindWidth, labelWidth := 0, 0
		for _, ref := range entry.report.Refs {
			kindWidth = max(kindWidth, text.UTF16Len(string(ref.Kind)))
			labelWidth = max(labelWidth, text.UTF16Len(label(ref)))
		}
		for _, ref := range entry.report.Refs {
			line := "  " + pad(string(ref.Kind), kindWidth) + "  " + pad(label(ref), labelWidth) + "  " + string(ref.Status)
			env.Log.Info(trimEnd(line))
			statuses[ref.Status]++
			paths++
		}
	}
	summary := fmt.Sprintf("%s, %s: %d in-game", plural(len(found), "model"), plural(paths, "path"), statuses[InGame]+statuses[InGameReplaced])
	failed := ""
	if len(unreadable) > 0 {
		failed = ", " + plural(len(unreadable), "model") + " unreadable"
	}
	if targets == nil {
		env.Log.Info(fmt.Sprintf("%s, %d custom%s.", summary, statuses[Custom], failed))
	} else {
		env.Log.Info(fmt.Sprintf("%s, %d custom imported, %d custom not imported%s.",
			summary, statuses[CustomImported], statuses[CustomNotImported], failed))
	}
	if len(unreadable) > 0 {
		return nil, &diag.Error{
			Msg:  plural(len(unreadable), "model") + " could not be read.",
			Hint: "Re-export or remove " + strings.Join(unreadable, ", ") + "; the report above lists why each one is unreadable.",
		}
	}
	if reports == nil {
		reports = []ModelReport{}
	}
	return reports, nil
}

// readModel reads the model the command line names, relative to the project folder.
func readModel(root, file string) (model, error) {
	path := fsx.Resolve(root, file)
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return model{}, &diag.Error{
			Msg:  file + " does not exist.",
			Hint: "Model paths are relative to the project folder, e.g. assets/Models/Knight.mdx.",
		}
	}
	if err != nil {
		// Reading a folder fails in a way that differs by system, so ask the file system directly.
		if fsx.IsDir(path) {
			return model{}, &diag.Error{Msg: file + " is a folder, not a model file."}
		}
		return model{}, &diag.Error{Msg: file + " could not be read: " + fsx.Reason(err), Cause: err}
	}
	heading := fsx.ToPosix(path)
	if inside, err := filepath.Rel(root, path); err == nil && !strings.HasPrefix(inside, "..") && !filepath.IsAbs(inside) {
		heading = fsx.ToPosix(inside)
	}
	return model{heading, data}, nil
}

// pad fills s with spaces to width, counted as JavaScript's padEnd counts.
func pad(s string, width int) string {
	if missing := width - text.UTF16Len(s); missing > 0 {
		return s + strings.Repeat(" ", missing)
	}
	return s
}

// trimEnd removes the white space at the end of a line, as JavaScript's trimEnd does.
func trimEnd(s string) string {
	return strings.TrimRightFunc(s, text.IsSpace)
}
