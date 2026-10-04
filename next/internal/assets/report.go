package assets

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/mdlsvensson/moonwell/next/internal/mapdir"
	"github.com/mdlsvensson/moonwell/next/internal/war3/model"
)

// PathStatus says where a file a model references comes from.
type PathStatus string

// Outside a project a path is InGame or Custom. In a project, where the files a build imports are known, it is
// one of the other four: InGameReplaced is a path the game ships and a build imports a file over.
const (
	InGame            PathStatus = "in-game path"
	InGameReplaced    PathStatus = "in-game path, replaced"
	CustomImported    PathStatus = "custom path, imported"
	CustomNotImported PathStatus = "custom path, not imported"
	Custom            PathStatus = "custom path"
)

// Model is a model file to report on.
type Model struct {
	Heading string // how the report names it
	Data    []byte
}

// Models returns the models among assets, with their headings: the files a build imports as .mdx or .mdl, in
// the order of the assets. A heading names the file a model is read from, under assets/ or in its library.
func Models(assets []Asset) []Model {
	var models []Model
	for _, asset := range assets {
		if isModel(asset.Target) {
			models = append(models, Model{Heading: headingOf(asset), Data: asset.Bytes})
		}
	}
	return models
}

// isModel reports whether an in-map path is a model's, by its extension in any letter case.
func isModel(target string) bool {
	key := mapdir.Key(target)
	return strings.HasSuffix(key, ".mdx") || strings.HasSuffix(key, ".mdl")
}

// headingOf names the file an asset is read from: under assets/, or in the library that ships it.
func headingOf(asset Asset) string {
	if asset.Library != "" {
		return "library " + asset.Library + ": " + asset.Source
	}
	return ownFolder + "/" + asset.Source
}

// ModelRef is one file a model references. Status is "" for a reference without a path, which is a replaceable
// texture such as the team colour.
type ModelRef struct {
	model.Path
	Status PathStatus
}

// ModelReport is the references of one model, or why there are none to list.
type ModelReport struct {
	Heading    string
	Refs       []ModelRef
	Unreadable string // why the model could not be read; "" when it could
}

// ReportModels lists the files each model references. gamePaths are the game's own; targets are the in-map
// paths a build imports, by mapdir.Key, or nil outside a project. A project that imports nothing has targets
// that are empty and not nil. A model that cannot be read is reported in its place, so that it does not hide
// the others.
func ReportModels(models []Model, gamePaths, targets map[string]bool) []ModelReport {
	reports := make([]ModelReport, len(models))
	for i, found := range models {
		reports[i] = reportOn(found, gamePaths, targets)
	}
	return reports
}

// reportOn reads one model's references and gives each that has a path its status. Every error of the reader
// becomes the line that says why the model is unreadable: this relies on model.Paths raising expected failures
// only, about the bytes it is given, and none that would be Moonwell's own fault.
func reportOn(found Model, gamePaths, targets map[string]bool) ModelReport {
	report := ModelReport{Heading: found.Heading}
	paths, err := model.Paths(found.Data, found.Heading)
	if err != nil {
		// The reader's message is without the file, which the heading names.
		report.Unreadable = err.Error()
		return report
	}
	report.Refs = make([]ModelRef, len(paths))
	for i, path := range paths {
		report.Refs[i] = ModelRef{Path: path}
		if path.Path != "" {
			report.Refs[i].Status = statusOf(path.Path, gamePaths, targets)
		}
	}
	return report
}

// statusOf is a reference's status: whether the game ships its path and, in a project (targets is not nil),
// whether a build imports it.
func statusOf(path string, gamePaths, targets map[string]bool) PathStatus {
	inGame := gamePaths[GamePathKey(path)]
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

// referenceKey is how a reference is compared with the paths a build imports: by mapdir.Key, with a requested
// .mdl as the .mdx the game loads in its place. An imported .mdl is never loaded, so the imports keep theirs.
func referenceKey(path string) string {
	return loadedModel(mapdir.Key(path))
}

// RenderReports is the report as lines: each model's heading and table, then the summary line. inProject says
// which statuses the references have, those of a project or those outside one, and so which the summary counts.
func RenderReports(reports []ModelReport, inProject bool) []string {
	var lines []string
	for _, report := range reports {
		lines = append(lines, report.Heading)
		lines = append(lines, report.body()...)
	}
	return append(lines, summaryOf(reports, inProject))
}

// body is the indented lines below a report's heading: why the model is unreadable, that it references
// nothing, or the table of its references.
func (r ModelReport) body() []string {
	switch {
	case r.Unreadable != "":
		return []string{"  (unreadable: " + r.Unreadable + ")"}
	case len(r.Refs) == 0:
		return []string{"  (no referenced files)"}
	}
	return table(r.Refs)
}

// table is a line for each reference: what the model uses it for, the path, and the status, in columns as wide
// as their longest entry. A reference without a status ends at its path.
func table(refs []ModelRef) []string {
	kindWidth, pathWidth := 0, 0
	for _, ref := range refs {
		kindWidth = max(kindWidth, utf8.RuneCountInString(string(ref.Kind)))
		pathWidth = max(pathWidth, utf8.RuneCountInString(shown(ref)))
	}
	lines := make([]string, len(refs))
	for i, ref := range refs {
		line := "  " + padded(string(ref.Kind), kindWidth) + "  " + padded(shown(ref), pathWidth) + "  " + string(ref.Status)
		lines[i] = strings.TrimRight(line, " ")
	}
	return lines
}

// shown is how the report shows a reference: its path with backslashes, as the game and World Editor write
// one, or the name of its slot.
func shown(ref ModelRef) string {
	return strings.ReplaceAll(model.Describe(ref.Path), "/", `\`)
}

// padded is text with spaces after it up to width characters.
func padded(text string, width int) string {
	return text + strings.Repeat(" ", max(0, width-utf8.RuneCountInString(text)))
}

// tally is what a summary counts: the references, those of each status, and the models that could not be read.
// A reference without a path counts among the references.
type tally struct {
	paths      int
	byStatus   map[PathStatus]int
	unreadable int
}

func tallyOf(reports []ModelReport) tally {
	count := tally{byStatus: map[PathStatus]int{}}
	for _, report := range reports {
		count.paths += len(report.Refs)
		for _, ref := range report.Refs {
			count.byStatus[ref.Status]++
		}
		if report.Unreadable != "" {
			count.unreadable++
		}
	}
	return count
}

// summaryOf is the last line of a report: how many models and references it lists, how many references have
// each status, and how many models could not be read.
func summaryOf(reports []ModelReport, inProject bool) string {
	count := tallyOf(reports)
	summary := fmt.Sprintf("%s, %s: %d in-game", counted(len(reports), "model"), counted(count.paths, "path"),
		count.byStatus[InGame]+count.byStatus[InGameReplaced])
	if inProject {
		summary += fmt.Sprintf(", %d custom imported, %d custom not imported",
			count.byStatus[CustomImported], count.byStatus[CustomNotImported])
	} else {
		summary += fmt.Sprintf(", %d custom", count.byStatus[Custom])
	}
	if count.unreadable > 0 {
		summary += ", " + counted(count.unreadable, "model") + " unreadable"
	}
	return summary + "."
}

// counted is a number with its noun, as "1 model" and "2 models".
func counted(count int, noun string) string {
	if count == 1 {
		return "1 " + noun
	}
	return strconv.Itoa(count) + " " + noun + "s"
}
