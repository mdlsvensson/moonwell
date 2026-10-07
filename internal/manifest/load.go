package manifest

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	moonwell "github.com/mdlsvensson/moonwell"
	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/env"
	"github.com/mdlsvensson/moonwell/internal/fsx"
)

// The files of a project that this package reads or names, from the project folder.
const (
	sharedManifest = "moonwell.pkl"
	localManifest  = "moonwell.local.pkl"
	pklProjectFile = "PklProject"
	depsFile       = "PklProject.deps.json"
)

// Load evaluates the project's manifest in e.Root with the pkl program and returns the project.
func Load(ctx context.Context, e *env.Env, pkl string) (*Project, error) {
	file, err := manifestFile(e.Root)
	if err != nil {
		return nil, err
	}
	if err := checkPackage(e.Root); err != nil {
		return nil, err
	}
	printed, err := evaluate(ctx, e, pkl, file)
	if err != nil {
		return nil, err
	}
	return Decode(e.Root, file, []byte(printed))
}

// Decode is the second half of Load: the JSON pkl printed, as a project. file is the manifest that was
// evaluated: errors name it, and an object written in it has it as its source. A field the structs do not have is
// passed over, because a later package of the same minor version may print one. JSON that does not fit the
// structs, or that lacks a text every project has, is no project, and is refused with one error.
func Decode(root, file string, data []byte) (*Project, error) {
	project := &Project{Root: root, File: file}
	if err := json.Unmarshal(data, project); err != nil {
		return nil, errNotAProject(file, strings.ReplaceAll(err.Error(), "json: ", ""), err)
	}
	if missing := project.missingTexts(); len(missing) > 0 {
		return nil, errNotAProject(file, "it has no "+diag.JoinWords(missing, "and", -1), nil)
	}
	project.Objects.nameSources(file)
	return project, nil
}

// missingTexts names the texts p lacks of the four the schema gives every project. JSON that fits the structs and
// has none of them is the output of something that is no manifest: a file that amends another module, or none.
// Nothing else of the shape is looked at here: it is Pkl's to check.
func (p *Project) missingTexts() []string {
	var missing []string
	for _, text := range []struct{ name, value string }{
		{"map.folder", p.Map.Folder}, {"map.entry", p.Map.Entry}, {"build.folder", p.Build.Folder},
		{"yue.version", p.Yue.Version},
	} {
		if text.value == "" {
			missing = append(missing, text.name)
		}
	}
	return missing
}

// manifestFile is the manifest to evaluate: the local one, which amends the shared one, when it exists.
func manifestFile(root string) (string, error) {
	for _, file := range []string{localManifest, sharedManifest} {
		if fsx.Exists(filepath.Join(root, file)) {
			return file, nil
		}
	}
	return "", errNoManifest(root)
}

// checkPackage fails unless the moonwell Pkl package the project resolved is of this program's version.
func checkPackage(root string) error {
	deps, err := os.ReadFile(filepath.Join(root, depsFile))
	if errors.Is(err, fs.ErrNotExist) {
		return errNoDeps()
	}
	if err != nil {
		return errDepsUnreadable(err)
	}
	version, err := ReadPackageVersion(deps)
	if err != nil {
		return err
	}
	return CheckPackageVersion(version, moonwell.Version)
}

// evaluate runs pkl on the manifest in the project folder and returns the JSON it printed.
func evaluate(ctx context.Context, e *env.Env, pkl, file string) (string, error) {
	args := []string{"eval", "--format", "json", "--project-dir", ".", file}
	result, err := e.Run(ctx, pkl, args, env.RunOptions{Dir: e.Root})
	if err != nil {
		return "", err
	}
	if result.Code != 0 {
		return "", errEvaluation(file, cmp.Or(result.Stderr, result.Stdout))
	}
	var value json.RawMessage
	if err := json.Unmarshal([]byte(result.Stdout), &value); err != nil {
		return "", errNotJSON(file, result.Stdout, err)
	}
	return result.Stdout, nil
}

// firstCharacters is the start of text, at most limit characters long.
func firstCharacters(text string, limit int) string {
	count := 0
	for i := range text {
		if count == limit {
			return text[:i]
		}
		count++
	}
	return text
}

// ---- errors ----

func errNoManifest(root string) error {
	return &diag.Error{
		Msg:  "No moonwell.pkl found in this directory.",
		File: root,
		Hint: "Run this command from a Moonwell project, or create one with `moonwell init <dir>`.",
	}
}

func errNoDeps() error {
	return &diag.Error{
		Msg:  "PklProject.deps.json is missing.",
		File: pklProjectFile,
		Hint: "Run `pkl project resolve` in the project folder.",
	}
}

func errDepsUnreadable(cause error) error {
	return &diag.Error{
		Msg:   "Reading PklProject.deps.json failed: " + fsx.Reason(cause),
		File:  depsFile,
		Hint:  "Check that it is a file this user may read, or write it again with `pkl project resolve`.",
		Cause: cause,
	}
}

// errEvaluation shows what pkl printed when it failed: pkl's own words name the line of the manifest.
func errEvaluation(file, output string) error {
	return &diag.Error{Msg: "Evaluating " + file + " failed:\n" + fsx.TrimASCIISpace(output), File: file}
}

func errNotJSON(file, output string, cause error) error {
	return &diag.Error{
		Msg:   "pkl eval printed output that is not valid JSON:\n" + firstCharacters(fsx.TrimASCIISpace(output), 500),
		File:  file,
		Hint:  "Check that pkl on PATH is Pkl 0.32 or newer and that no other program is named pkl.",
		Cause: cause,
	}
}

// errNotAProject is the failure for JSON that is not shaped like a project; reason says in what. Pkl checks the
// shape of a manifest that amends the schema, so this manifest does not amend it, or amends the schema of another
// version.
func errNotAProject(file, reason string, cause error) error {
	return &diag.Error{
		Msg:  file + " does not evaluate to a Moonwell project: " + reason,
		File: file,
		Hint: "moonwell.pkl must amend \"@moonwell/Project.pkl\", and the moonwell package in PklProject must be of " +
			"this program's version, " + moonwell.Version + ".",
		Cause: cause,
	}
}
