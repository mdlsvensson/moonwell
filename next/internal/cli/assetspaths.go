package cli

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mdlsvensson/moonwell/next/internal/assets"
	"github.com/mdlsvensson/moonwell/next/internal/build"
	"github.com/mdlsvensson/moonwell/next/internal/diag"
	"github.com/mdlsvensson/moonwell/next/internal/env"
	"github.com/mdlsvensson/moonwell/next/internal/fsx"
	"github.com/mdlsvensson/moonwell/next/internal/mapdir"
	"github.com/mdlsvensson/moonwell/next/internal/war3/model"
)

// runAssetsPaths is `moonwell assets:paths [file]`: it lists the files a model references, and says of each
// whether the game ships its path. In a project it also says whether a build imports a file at the path.
func runAssetsPaths(ctx context.Context, e *env.Env, c call) error {
	file := "" // no file: every model among the project's assets
	if arguments := c.said.arguments(); len(arguments) > 0 {
		file = arguments[0]
	}
	return assetsPaths(ctx, e, file, assets.LoadGamePaths())
}

// assetsPaths is assets:paths for a file, "" for none, with gamePaths as the keys of the paths the game ships.
//
// A folder with a moonwell.pkl is a project. There the command first works out what a build imports, which
// syncs the libraries. It then reports on the model at file, read from the folder the command runs in, or
// without a file on every model among the imported files; outside a project it needs a file. The report is
// logged, and after it, and only then, a model among the imported files that could not be read fails the
// command.
func assetsPaths(ctx context.Context, e *env.Env, file string, gamePaths map[string]bool) error {
	inProject := fsx.Exists(filepath.Join(e.Root, "moonwell.pkl"))
	var imported []assets.Asset
	var targets map[string]bool // the in-map paths a build imports; nil outside a project, which has no build
	if inProject {
		found, err := importedByABuild(ctx, e)
		if err != nil {
			return err
		}
		imported, targets = found, targetsOf(found)
	}
	models, err := modelsToReport(e.Root, file, inProject, imported)
	if err != nil {
		return err
	}
	if len(models) == 0 {
		e.Log.Info("No models under assets/.")
		return nil
	}
	if len(gamePaths) == 0 {
		e.Log.Warn("Moonwell's in-game path list is empty, so every path shows as custom.")
	}
	if file != "" {
		// A file the line names is refused when it is no model, in the words of the model's reader; a model among
		// the imported files is reported in its place, with the reason.
		if _, err := model.Paths(models[0].Data, models[0].Heading); err != nil {
			return err
		}
	}
	reports := assets.ReportModels(models, gamePaths, targets)
	for _, line := range assets.RenderReports(reports, inProject) {
		e.Log.Info(line)
	}
	return refuseUnreadable(reports)
}

// importedByABuild is what a build of the project in e.Root imports: the map's own assets and the files the
// libraries ship. It evaluates the manifest and syncs the libraries, and holds the build lock while it does.
func importedByABuild(ctx context.Context, e *env.Env) ([]assets.Asset, error) {
	// Before the lock, as a build has it: a manifest that does not load makes no dist folder.
	p, err := build.Load(ctx, e)
	if err != nil {
		return nil, err
	}
	release, err := build.Acquire(e.Root)
	if err != nil {
		return nil, err
	}
	defer release()
	found, _, err := syncedAssets(ctx, e, p)
	return found, err
}

// targetsOf is the in-map paths the assets are imported as, by mapdir.Key. For a project that imports nothing
// it is empty and not nil: the report tells a project from a folder that is none by that.
func targetsOf(imported []assets.Asset) map[string]bool {
	targets := map[string]bool{}
	for _, asset := range imported {
		targets[mapdir.Key(asset.Target)] = true
	}
	return targets
}

// modelsToReport is the models the command reports on: the one at file, or without a file every model among the
// files a build imports, which may be none. Outside a project there are no such files, and the command needs a
// file.
func modelsToReport(root, file string, inProject bool, imported []assets.Asset) ([]assets.Model, error) {
	switch {
	case file != "":
		named, err := namedModel(root, file)
		if err != nil {
			return nil, err
		}
		return []assets.Model{named}, nil
	case !inProject:
		return nil, errNeedsAModel()
	}
	return assets.Models(imported), nil
}

// namedModel reads the model the command line names, a path from root or a whole path. Whether its bytes are a
// model is not looked at here.
func namedModel(root, file string) (assets.Model, error) {
	path := fsx.Resolve(root, file)
	data, err := os.ReadFile(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return assets.Model{}, errNoSuchModel(file)
	// What a system says of reading a folder differs from system to system, so the folder is found by a look.
	case err != nil && fsx.IsDir(path):
		return assets.Model{}, errModelIsAFolder(file)
	case err != nil:
		return assets.Model{}, errModelNotRead(file, err)
	}
	return assets.Model{Heading: headingOf(root, path), Data: data}, nil
}

// headingOf is how the report names the model at path: by its path from root, with "/", when that does not
// start with "..", and else by its whole path.
func headingOf(root, path string) string {
	below, err := filepath.Rel(root, path)
	if err != nil || strings.HasPrefix(below, "..") || filepath.IsAbs(below) {
		return fsx.ToPosix(path)
	}
	return fsx.ToPosix(below)
}

// refuseUnreadable fails when the report has a model that could not be read, and names each. The report says why
// each could not: it is logged in full before this failure.
func refuseUnreadable(reports []assets.ModelReport) error {
	var unreadable []string
	for _, report := range reports {
		if report.Unreadable != "" {
			unreadable = append(unreadable, report.Heading)
		}
	}
	if len(unreadable) == 0 {
		return nil
	}
	return errUnreadableModels(unreadable)
}

// ---- errors ----

func errNeedsAModel() error {
	return &diag.Error{
		Msg:  "assets:paths needs a model file outside a Moonwell project.",
		Hint: "moonwell assets:paths assets/Models/Knight.mdx",
	}
}

// errNoSuchModel, errModelIsAFolder and errModelNotRead name the file as the command line wrote it.
func errNoSuchModel(file string) error {
	return &diag.Error{
		Msg:  file + " does not exist.",
		Hint: "Model paths are relative to the project folder, e.g. assets/Models/Knight.mdx.",
	}
}

func errModelIsAFolder(file string) error {
	return &diag.Error{Msg: file + " is a folder, not a model file."}
}

func errModelNotRead(file string, cause error) error {
	return &diag.Error{Msg: file + " could not be read: " + fsx.Reason(cause), Cause: cause}
}

// errUnreadableModels names the models by the headings the report gives them.
func errUnreadableModels(headings []string) error {
	count := strconv.Itoa(len(headings)) + " models"
	if len(headings) == 1 {
		count = "1 model"
	}
	return &diag.Error{
		Msg: count + " could not be read.",
		Hint: "Re-export or remove " + strings.Join(headings, ", ") + "; the report above lists why each one is " +
			"unreadable.",
	}
}
