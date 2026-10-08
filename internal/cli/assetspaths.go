package cli

import (
	"context"
	"strconv"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/assets"
	"github.com/mdlsvensson/moonwell/internal/build"
	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/env"
	"github.com/mdlsvensson/moonwell/internal/manifest"
	"github.com/mdlsvensson/moonwell/internal/war3/model"
)

func runAssetsPaths(ctx context.Context, e *env.Env, c commandArgs) error {
	file := ""
	if len(c.arguments) > 0 {
		file = c.arguments[0]
	}
	return reportAssetPaths(ctx, e, file, assets.LoadGamePaths())
}

func reportAssetPaths(ctx context.Context, e *env.Env, file string, gamePaths map[string]bool) error {
	inProject := manifest.IsProject(e.Root)
	var imported []assets.Asset
	var targets map[string]bool
	if inProject {
		found, err := assetsOfBuild(ctx, e)
		if err != nil {
			return err
		}
		imported, targets = found, assets.TargetSet(found)
	}
	models, err := selectModels(e.Root, file, inProject, imported)
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
		if _, err := model.ReadPaths(models[0].Data, models[0].Heading); err != nil {
			return err
		}
	}
	reports := assets.ReportModels(models, gamePaths, targets)
	for _, line := range assets.RenderReports(reports, inProject) {
		e.Log.Info(line)
	}
	return checkModelsReadable(reports)
}

func assetsOfBuild(ctx context.Context, e *env.Env) ([]assets.Asset, error) {
	p, err := build.Load(ctx, e)
	if err != nil {
		return nil, err
	}
	release, err := build.AcquireLock(e.Root)
	if err != nil {
		return nil, err
	}
	defer release()
	found, _, err := collectSyncedAssets(ctx, e, p)
	return found, err
}

func selectModels(root, file string, inProject bool, imported []assets.Asset) ([]assets.Model, error) {
	switch {
	case file != "":
		named, err := assets.ReadModel(root, file)
		if err != nil {
			return nil, err
		}
		return []assets.Model{named}, nil
	case !inProject:
		return nil, errNeedsAModel()
	}
	return assets.ModelsAmong(imported), nil
}

func checkModelsReadable(reports []assets.ModelReport) error {
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

func errNeedsAModel() error {
	return &diag.Error{
		Msg:  "assets:paths needs a model file outside a Moonwell project.",
		Hint: "moonwell assets:paths assets/Models/Knight.mdx",
	}
}

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
