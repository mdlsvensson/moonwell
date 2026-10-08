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

const (
	SharedFile = "moonwell.pkl"
	LocalFile  = "moonwell.local.pkl"
)

const (
	pklProjectFile = "PklProject"
	depsFile       = "PklProject.deps.json"
)

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

func Decode(root, file string, data []byte) (*Project, error) {
	project := &Project{Root: root, File: file}
	if err := json.Unmarshal(data, project); err != nil {
		return nil, errNotAProject(file, reasonOf(err), err)
	}
	if missing := project.missingTexts(); len(missing) > 0 {
		return nil, errNotAProject(file, "it has no "+diag.JoinWords(missing, "and", -1), nil)
	}
	project.Objects.nameSources(file)
	return project, nil
}

func reasonOf(err error) string {
	var mismatch *json.UnmarshalTypeError
	if !errors.As(err, &mismatch) {
		return strings.ReplaceAll(err.Error(), "json: ", "")
	}
	where := strings.TrimSuffix(err.Error(), mismatch.Error()) + cmp.Or(mismatch.Field, "the value")
	if number, written := strings.CutPrefix(mismatch.Value, "number "); written {
		return where + ": " + number + " is no number that fits there"
	}
	return where + " is of the wrong kind (" + mismatch.Value + ")"
}

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

func IsProject(root string) bool {
	return fsx.Exists(filepath.Join(root, SharedFile))
}

func manifestFile(root string) (string, error) {
	for _, file := range []string{LocalFile, SharedFile} {
		if fsx.Exists(filepath.Join(root, file)) {
			return file, nil
		}
	}
	return "", errNoManifest(root)
}

func checkPackage(root string) error {
	deps, err := os.ReadFile(filepath.Join(root, depsFile))
	if errors.Is(err, fs.ErrNotExist) {
		return errNoDeps()
	}
	if err != nil {
		return errDepsUnreadable(err)
	}
	version, err := readPackageVersion(deps)
	if err != nil {
		return err
	}
	return checkPackageVersion(version, moonwell.Version)
}

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

func errNotAProject(file, reason string, cause error) error {
	return &diag.Error{
		Msg:  file + " does not evaluate to a Moonwell project: " + reason,
		File: file,
		Hint: "moonwell.pkl must amend \"@moonwell/Project.pkl\", and the moonwell package in PklProject must be of " +
			"this program's version, " + moonwell.Version + ".",
		Cause: cause,
	}
}
