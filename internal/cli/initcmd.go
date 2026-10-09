package cli

import (
	"cmp"
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"syscall"

	moonwell "github.com/mdlsvensson/moonwell"
	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/env"
	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/manifest"
	"github.com/mdlsvensson/moonwell/internal/toolchain"
)

func runInit(ctx context.Context, e *env.Env, args commandArgs) error {
	arguments := args.arguments
	if len(arguments) == 0 || arguments[0] == "" {
		return errInitNeedsAFolder()
	}
	schemaDir := ""
	if args.link {
		checkout, err := findCheckout(e.Root)
		if err != nil {
			return err
		}
		schemaDir = filepath.Join(checkout, "schema")
	}
	return createProject(ctx, e, arguments[0], schemaDir)
}

func findCheckout(start string) (string, error) {
	for dir := start; ; dir = filepath.Dir(dir) {
		if data, err := os.ReadFile(filepath.Join(dir, "go.mod")); err == nil && moduleLine.Match(data) {
			return dir, nil
		}
		if dir == filepath.Dir(dir) {
			return "", errNoCheckout()
		}
	}
}

var moduleLine = regexp.MustCompile(`(?m)^module\s+github\.com/mdlsvensson/moonwell\s*$`)

func createProject(ctx context.Context, e *env.Env, dir, schemaDir string) error {
	projectDir := fsx.ResolvePath(e.Root, dir)
	existed, err := checkNewOrEmpty(projectDir, dir)
	if err != nil {
		return err
	}
	schemaRelPath, err := linkedSchemaDir(projectDir, schemaDir)
	if err != nil {
		return err
	}
	pkl, err := toolchain.FindPkl(ctx, e)
	if err != nil {
		return err
	}
	err = writeProject(projectDir, dir, schemaRelPath)
	if err == nil {
		err = resolvePklProject(ctx, e, pkl, projectDir, dir)
	}
	if err != nil {
		undoInit(projectDir, existed)
		return err
	}
	e.Log.Info("Created " + dir + ". Check launch.gameExecutable in moonwell.local.pkl, then: cd " + dir +
		" && moonwell build")
	return nil
}

func checkNewOrEmpty(projectDir, displayPath string) (existed bool, err error) {
	info, err := os.Stat(projectDir)
	switch {
	case errors.Is(err, fs.ErrNotExist), errors.Is(err, syscall.ENOTDIR):
		return false, checkNotBrokenSymlink(projectDir, displayPath)
	case err != nil:
		return false, errFolderNotRead(displayPath, err)
	case !info.IsDir():
		return false, errNotAFolder(displayPath)
	}
	entries, err := os.ReadDir(projectDir)
	switch {
	case err != nil:
		return false, errFolderNotRead(displayPath, err)
	case len(entries) > 0:
		return false, errNotEmpty(displayPath)
	}
	return true, nil
}

func checkNotBrokenSymlink(projectDir, displayPath string) error {
	if info, err := fsx.Lstat(projectDir); err == nil && info != nil {
		return errNotAFolder(displayPath)
	}
	return nil
}

func linkedSchemaDir(projectDir, schemaDir string) (string, error) {
	if schemaDir == "" {
		return "", nil
	}
	return relSchemaDir(projectDir, schemaDir)
}

func relSchemaDir(projectDir, schemaDir string) (string, error) {
	rel, err := filepath.Rel(projectDir, schemaDir)
	if err != nil || filepath.IsAbs(rel) {
		return "", errAnotherDrive(schemaDir)
	}
	return fsx.ToSlash(rel), nil
}

func writeProject(projectDir, displayPath, schemaRelPath string) error {
	files, err := moonwell.TemplateFiles()
	if err != nil {
		return err
	}
	files = append(files,
		moonwell.TemplateFile{Path: "PklProject", Data: []byte(manifest.PklProjectText(moonwell.Version, schemaRelPath))},
		moonwell.TemplateFile{Path: manifest.LocalManifest, Data: []byte(manifest.LocalManifestText())},
	)
	for _, file := range files {
		if err := writeProjectFile(projectDir, displayPath, file); err != nil {
			return err
		}
	}
	return nil
}

func writeProjectFile(projectDir, displayPath string, file moonwell.TemplateFile) error {
	fullPath := filepath.Join(projectDir, filepath.FromSlash(file.Path))
	err := os.MkdirAll(filepath.Dir(fullPath), 0o777)
	if err == nil {
		err = os.WriteFile(fullPath, file.Data, 0o666)
	}
	if err != nil {
		return errNotWritten(displayPathOf(displayPath, file.Path), err)
	}
	return nil
}

func displayPathOf(dir, name string) string {
	return fsx.ToSlash(filepath.Join(dir, filepath.FromSlash(name)))
}

func resolvePklProject(ctx context.Context, e *env.Env, pkl, projectDir, displayPath string) error {
	result, err := e.Run(ctx, pkl, []string{"project", "resolve"}, env.RunOptions{Dir: projectDir})
	if err != nil {
		return err
	}
	if result.ExitCode != 0 {
		return errNotResolved(displayPath, cmp.Or(result.Stderr, result.Stdout))
	}
	return nil
}

func undoInit(projectDir string, existed bool) {
	if !existed {
		_ = fsx.RemoveAll(projectDir)
		return
	}
	entries, err := os.ReadDir(projectDir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		_ = fsx.RemoveAll(filepath.Join(projectDir, entry.Name()))
	}
}

const chooseEmpty = "Choose a new or empty directory."

func errInitNeedsAFolder() error {
	return &diag.Error{Msg: "init needs a directory.", Hint: "moonwell init my-map"}
}

func errNotAFolder(dir string) error {
	return &diag.Error{Msg: dir + " is not a directory.", Hint: chooseEmpty}
}

func errNotEmpty(dir string) error {
	return &diag.Error{Msg: dir + " is not empty.", Hint: chooseEmpty}
}

func errFolderNotRead(dir string, cause error) error {
	return &diag.Error{
		Msg:   "Reading " + dir + " failed: " + fsx.Reason(cause),
		File:  dir,
		Hint:  "Choose a new directory, or an empty one that you may read.",
		Cause: cause,
	}
}

func errNotWritten(path string, cause error) error {
	return &diag.Error{
		Msg:   "Writing " + path + " failed: " + fsx.Reason(cause),
		File:  path,
		Hint:  "Choose a directory that you may write to, on a disk with room for the project.",
		Cause: cause,
	}
}

func errNotResolved(dir, output string) error {
	return &diag.Error{
		Msg:  "pkl project resolve failed:\n" + fsx.TrimASCIISpace(output),
		File: displayPathOf(dir, "PklProject"),
		Hint: "Pkl says why above. Without --link it fetches the moonwell package from GitHub: check the network " +
			"connection, then run init again.",
	}
}

func errNoCheckout() error {
	return &diag.Error{
		Msg:  "--link only works when Moonwell runs from a local checkout.",
		Hint: "Run it in the folder of a Moonwell checkout, or below it.",
	}
}

func errAnotherDrive(schemaDir string) error {
	return &diag.Error{
		Msg: "--link needs the project on the same drive as this Moonwell checkout.",
		Hint: "Create the project on " + filepath.VolumeName(schemaDir) + `\` +
			" (Pkl cannot load a local dependency from another drive), or use the published package with a plain " +
			"`init`.",
	}
}
