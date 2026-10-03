package cli

import (
	"context"
	"os"
	"path/filepath"
	"regexp"

	moonwell "github.com/mdlsvensson/moonwell"
	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/pipeline"
	"github.com/mdlsvensson/moonwell/internal/proc"
	"github.com/mdlsvensson/moonwell/internal/project"
	"github.com/mdlsvensson/moonwell/internal/text"
)

// InitOptions say how a project is created.
type InitOptions struct {
	// Link makes the project use the Pkl package of a Moonwell checkout instead of the published one.
	Link bool
	// Checkout is that checkout's folder; "" finds it from the working directory.
	Checkout string
}

const chooseEmpty = "Choose a new or empty directory."

// Init scaffolds a project into dir (new or empty) and resolves its Pkl dependencies. A failed init leaves nothing.
// It returns the project's folder.
func Init(ctx context.Context, env *pipeline.Env, dir string, options InitOptions) (string, error) {
	target, err := filepath.Abs(fsx.Resolve(env.Root, dir))
	if err != nil {
		return "", err
	}
	existed := fsx.Exists(target)
	if existed {
		if !fsx.IsDir(target) {
			return "", &diag.Error{Msg: dir + " is not a directory.", Hint: chooseEmpty}
		}
		entries, err := os.ReadDir(target)
		if err != nil {
			return "", err
		}
		if len(entries) > 0 {
			return "", &diag.Error{Msg: dir + " is not empty.", Hint: chooseEmpty}
		}
	}
	local := ""
	if options.Link {
		if local, err = localSchema(target, env.Root, options.Checkout); err != nil {
			return "", err
		}
	}
	pkl, err := env.Pkl(ctx)
	if err != nil {
		return "", err
	}
	if err := writeProject(ctx, env, target, dir, local, pkl); err != nil {
		undoInit(target, existed)
		return "", err
	}
	env.Log.Info("Created " + dir + ". Check launch.gameExecutable in moonwell.local.pkl, then: cd " + dir + " && moonwell build")
	return target, nil
}

func writeProject(ctx context.Context, env *pipeline.Env, target, dir, local, pkl string) error {
	files, err := moonwell.TemplateFiles()
	if err != nil {
		return err
	}
	files = append(files,
		moonwell.TemplateFile{Path: "PklProject", Data: []byte(project.PklProject(moonwell.Version, local))},
		moonwell.TemplateFile{Path: "moonwell.local.pkl", Data: []byte(project.LocalPkl())},
	)
	for _, file := range files {
		path := filepath.Join(target, filepath.FromSlash(file.Path))
		if err := os.MkdirAll(filepath.Dir(path), 0o777); err != nil {
			return err
		}
		if err := os.WriteFile(path, file.Data, 0o666); err != nil {
			return err
		}
	}
	result, err := env.Run(ctx, pkl, []string{"project", "resolve"}, proc.Options{Dir: target})
	if err != nil {
		return err
	}
	if result.Code != 0 {
		output := result.Stderr
		if output == "" {
			output = result.Stdout
		}
		return &diag.Error{Msg: "pkl project resolve failed:\n" + text.Trim(output), File: filepath.Join(dir, "PklProject")}
	}
	return nil
}

// undoInit removes what init wrote: the whole folder if init created it, else only its contents (it was empty). It
// does what it can: the failure that led here matters more than one of its own.
func undoInit(target string, existed bool) {
	if !existed {
		fsx.RemoveAll(target)
		return
	}
	entries, err := os.ReadDir(target)
	if err != nil {
		return
	}
	for _, entry := range entries {
		fsx.RemoveAll(filepath.Join(target, entry.Name()))
	}
}

var moduleLine = regexp.MustCompile(`(?m)^module\s+github\.com/mdlsvensson/moonwell\s*$`)

// localSchema is how a linked project refers to a checkout's schema/ folder. The checkout is the one named, or the
// folder at or above start whose go.mod names this module.
func localSchema(target, start, checkout string) (string, error) {
	if checkout == "" {
		for dir := start; ; dir = filepath.Dir(dir) {
			if data, err := os.ReadFile(filepath.Join(dir, "go.mod")); err == nil && moduleLine.Match(data) {
				checkout = dir
				break
			}
			if dir == filepath.Dir(dir) {
				return "", &diag.Error{
					Msg:  "--link only works when Moonwell runs from a local checkout.",
					Hint: "Run it in the folder of a Moonwell checkout, or below it.",
				}
			}
		}
	}
	return LinkPath(target, filepath.Join(checkout, "schema"))
}

// LinkPath is how a linked project in target refers to path in a checkout: always a relative path. On Windows there
// is none across drives, and Pkl cannot load a local dependency from another drive (PklProject.deps.json cannot
// record the path), so that fails.
func LinkPath(target, path string) (string, error) {
	inside, err := filepath.Rel(target, path)
	if err != nil || filepath.IsAbs(inside) {
		return "", &diag.Error{
			Msg: "--link needs the project on the same drive as this Moonwell checkout.",
			Hint: "Create the project on " + filepath.VolumeName(path) + `\` + " (Pkl cannot load a local dependency from another drive), " +
				"or use the published package with a plain `init`.",
		}
	}
	return fsx.ToPosix(inside), nil
}
