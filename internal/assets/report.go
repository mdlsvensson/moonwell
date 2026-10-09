package assets

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/mapdir"
	"github.com/mdlsvensson/moonwell/internal/war3/model"
)

type PathStatus string

const (
	InGame            PathStatus = "in-game path"
	InGameReplaced    PathStatus = "in-game path, replaced"
	CustomImported    PathStatus = "custom path, imported"
	CustomNotImported PathStatus = "custom path, not imported"
	Custom            PathStatus = "custom path"
)

type Model struct {
	Heading string
	Data    []byte
}

func ModelsAmong(assets []Asset) []Model {
	var models []Model
	for _, asset := range assets {
		if isModel(asset.Target) {
			models = append(models, Model{Heading: headingOf(asset), Data: asset.Data})
		}
	}
	return models
}

func isModel(target string) bool {
	key := mapdir.Key(target)
	return strings.HasSuffix(key, ".mdx") || strings.HasSuffix(key, ".mdl")
}

func headingOf(asset Asset) string {
	if asset.Library != "" {
		return "library " + asset.Library + ": " + asset.Source
	}
	return projectAssetsDir + "/" + asset.Source
}

func ReadModel(root, file string) (Model, error) {
	fullPath := fsx.ResolvePath(root, file)
	heading := displayPathOf(root, fullPath)
	data, err := os.ReadFile(fullPath)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return Model{}, errNoSuchModel(heading)
	case err != nil && fsx.IsDir(fullPath):
		return Model{}, errModelIsAFolder(heading)
	case err != nil:
		return Model{}, errModelNotRead(heading, err)
	}
	return Model{Heading: heading, Data: data}, nil
}

func TargetSet(imported []Asset) map[string]bool {
	targets := map[string]bool{}
	for _, asset := range imported {
		targets[mapdir.Key(asset.Target)] = true
	}
	return targets
}

type ModelRef struct {
	model.Path
	Status PathStatus
}

type ModelReport struct {
	Heading    string
	Refs       []ModelRef
	Unreadable string
}

func ReportModels(models []Model, gamePaths, targets map[string]bool) []ModelReport {
	reports := make([]ModelReport, len(models))
	for i, modelFile := range models {
		reports[i] = reportModel(modelFile, gamePaths, targets)
	}
	return reports
}

func reportModel(modelFile Model, gamePaths, targets map[string]bool) ModelReport {
	report := ModelReport{Heading: modelFile.Heading}
	paths, err := model.ReadPaths(modelFile.Data, modelFile.Heading)
	if err != nil {
		report.Unreadable = err.Error()
		return report
	}
	report.Refs = make([]ModelRef, len(paths))
	for i, path := range paths {
		report.Refs[i] = ModelRef{Path: path}
		if path.Path != "" {
			report.Refs[i].Status = pathStatus(path.Path, gamePaths, targets)
		}
	}
	return report
}

func pathStatus(path string, gamePaths, targets map[string]bool) PathStatus {
	inGame := gamePaths[gamePathKey(path)]
	imported := targets[referenceKey(path)]
	switch {
	case targets == nil && inGame:
		return InGame
	case targets == nil:
		return Custom
	case inGame && imported:
		return InGameReplaced
	case inGame:
		return InGame
	case imported:
		return CustomImported
	}
	return CustomNotImported
}

func referenceKey(path string) string {
	return loadedModelKey(mapdir.Key(path))
}

func RenderReports(reports []ModelReport, inProject bool) []string {
	var lines []string
	for _, report := range reports {
		lines = append(lines, report.Heading)
		lines = append(lines, report.lines()...)
	}
	return append(lines, formatSummary(reports, inProject))
}

func (r ModelReport) lines() []string {
	switch {
	case r.Unreadable != "":
		return []string{"  (unreadable: " + r.Unreadable + ")"}
	case len(r.Refs) == 0:
		return []string{"  (no referenced files)"}
	}
	return formatTable(r.Refs)
}

func formatTable(refs []ModelRef) []string {
	kindWidth, pathWidth := 0, 0
	for _, ref := range refs {
		kindWidth = max(kindWidth, utf8.RuneCountInString(string(ref.Kind)))
		pathWidth = max(pathWidth, utf8.RuneCountInString(formatRef(ref)))
	}
	lines := make([]string, len(refs))
	for i, ref := range refs {
		line := "  " + padRight(string(ref.Kind), kindWidth) + "  " + padRight(formatRef(ref), pathWidth) + "  " + string(ref.Status)
		lines[i] = strings.TrimRight(line, " ")
	}
	return lines
}

func formatRef(ref ModelRef) string {
	return strings.ReplaceAll(model.DescribePath(ref.Path), "/", `\`)
}

func padRight(text string, width int) string {
	return text + strings.Repeat(" ", max(0, width-utf8.RuneCountInString(text)))
}

type reportCounts struct {
	paths      int
	byStatus   map[PathStatus]int
	unreadable int
}

func countReports(reports []ModelReport) reportCounts {
	counts := reportCounts{byStatus: map[PathStatus]int{}}
	for _, report := range reports {
		counts.paths += len(report.Refs)
		for _, ref := range report.Refs {
			counts.byStatus[ref.Status]++
		}
		if report.Unreadable != "" {
			counts.unreadable++
		}
	}
	return counts
}

func formatSummary(reports []ModelReport, inProject bool) string {
	counts := countReports(reports)
	summary := fmt.Sprintf("%s, %s: %d in-game", pluralize(len(reports), "model"), pluralize(counts.paths, "path"),
		counts.byStatus[InGame]+counts.byStatus[InGameReplaced])
	if inProject {
		summary += fmt.Sprintf(", %d custom imported, %d custom not imported",
			counts.byStatus[CustomImported], counts.byStatus[CustomNotImported])
	} else {
		summary += fmt.Sprintf(", %d custom", counts.byStatus[Custom])
	}
	if counts.unreadable > 0 {
		summary += ", " + pluralize(counts.unreadable, "model") + " unreadable"
	}
	return summary + "."
}

func pluralize(count int, noun string) string {
	if count == 1 {
		return "1 " + noun
	}
	return strconv.Itoa(count) + " " + noun + "s"
}

func errNoSuchModel(displayPath string) error {
	return &diag.Error{
		Msg:  "This model file does not exist.",
		File: displayPath,
		Hint: "A model's path starts at the folder the command runs in, e.g. assets/Models/Knight.mdx.",
	}
}

func errModelIsAFolder(displayPath string) error {
	return &diag.Error{
		Msg:  "This is a folder, not a model file.",
		File: displayPath,
		Hint: "Name one model, e.g. assets/Models/Knight.mdx. In a project, assets:paths without a file reports " +
			"on every model among the assets.",
	}
}

func errModelNotRead(displayPath string, cause error) error {
	return &diag.Error{
		Msg:   "Reading the model failed: " + fsx.Reason(cause),
		File:  displayPath,
		Hint:  "Close any program that has the file open and check that it is a file you may read, then try again.",
		Cause: cause,
	}
}
