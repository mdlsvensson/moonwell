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

func runInit(ctx context.Context, e *env.Env, c call) error {
	arguments := c.arguments
	if len(arguments) == 0 || arguments[0] == "" {
		return errInitNeedsAFolder()
	}
	schema := ""
	if c.link {
		checkout, err := checkoutAbove(e.Root)
		if err != nil {
			return err
		}
		schema = filepath.Join(checkout, "schema")
	}
	return createProject(ctx, e, arguments[0], schema)
}

func createProject(ctx context.Context, e *env.Env, dir, schema string) error {
	target := fsx.ResolvePath(e.Root, dir)
	existed, err := newOrEmpty(target, dir)
	if err != nil {
		return err
	}
	local, err := linkTo(target, schema)
	if err != nil {
		return err
	}
	pkl, err := toolchain.PklProgram(ctx, e)
	if err != nil {
		return err
	}
	err = writeProject(target, dir, local)
	if err == nil {
		err = resolve(ctx, e, pkl, target, dir)
	}
	if err != nil {
		undoInit(target, existed)
		return err
	}
	e.Log.Info("Created " + dir + ". Check launch.gameExecutable in moonwell.local.pkl, then: cd " + dir +
		" && moonwell build")
	return nil
}

func newOrEmpty(target, dir string) (existed bool, err error) {
	info, err := os.Stat(target)
	switch {
	case errors.Is(err, fs.ErrNotExist), errors.Is(err, syscall.ENOTDIR):
		return false, refuseLinkToNothing(target, dir)
	case err != nil:
		return false, errFolderNotRead(dir, err)
	case !info.IsDir():
		return false, errNotAFolder(dir)
	}
	entries, err := os.ReadDir(target)
	switch {
	case err != nil:
		return false, errFolderNotRead(dir, err)
	case len(entries) > 0:
		return false, errNotEmpty(dir)
	}
	return true, nil
}

func refuseLinkToNothing(target, dir string) error {
	if info, err := fsx.Lstat(target); err == nil && info != nil {
		return errNotAFolder(dir)
	}
	return nil
}

func writeProject(target, dir, local string) error {
	files, err := moonwell.TemplateFiles()
	if err != nil {
		return err
	}
	files = append(files,
		moonwell.TemplateFile{Path: "PklProject", Data: []byte(manifest.PklProjectText(moonwell.Version, local))},
		moonwell.TemplateFile{Path: manifest.LocalManifest, Data: []byte(manifest.LocalManifestText())},
	)
	for _, file := range files {
		if err := writeProjectFile(target, dir, file); err != nil {
			return err
		}
	}
	return nil
}

func writeProjectFile(target, dir string, file moonwell.TemplateFile) error {
	path := filepath.Join(target, filepath.FromSlash(file.Path))
	err := os.MkdirAll(filepath.Dir(path), 0o777)
	if err == nil {
		err = os.WriteFile(path, file.Data, 0o666)
	}
	if err != nil {
		return errNotWritten(fileOf(dir, file.Path), err)
	}
	return nil
}

func fileOf(dir, name string) string {
	return fsx.ToSlash(filepath.Join(dir, filepath.FromSlash(name)))
}

func resolve(ctx context.Context, e *env.Env, pkl, target, dir string) error {
	result, err := e.Run(ctx, pkl, []string{"project", "resolve"}, env.RunOptions{Dir: target})
	if err != nil {
		return err
	}
	if result.ExitCode != 0 {
		return errNotResolved(dir, cmp.Or(result.Stderr, result.Stdout))
	}
	return nil
}

func undoInit(target string, existed bool) {
	if !existed {
		_ = fsx.RemoveAll(target)
		return
	}
	entries, err := os.ReadDir(target)
	if err != nil {
		return
	}
	for _, entry := range entries {
		_ = fsx.RemoveAll(filepath.Join(target, entry.Name()))
	}
}

var moduleLine = regexp.MustCompile(`(?m)^module\s+github\.com/mdlsvensson/moonwell\s*$`)

func checkoutAbove(start string) (string, error) {
	for dir := start; ; dir = filepath.Dir(dir) {
		if data, err := os.ReadFile(filepath.Join(dir, "go.mod")); err == nil && moduleLine.Match(data) {
			return dir, nil
		}
		if dir == filepath.Dir(dir) {
			return "", errNoCheckout()
		}
	}
}

func linkTo(target, schema string) (string, error) {
	if schema == "" {
		return "", nil
	}
	return linkPath(target, schema)
}

func linkPath(target, schema string) (string, error) {
	inside, err := filepath.Rel(target, schema)
	if err != nil || filepath.IsAbs(inside) {
		return "", errAnotherDrive(schema)
	}
	return fsx.ToSlash(inside), nil
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

func errNotWritten(file string, cause error) error {
	return &diag.Error{
		Msg:   "Writing " + file + " failed: " + fsx.Reason(cause),
		File:  file,
		Hint:  "Choose a directory that you may write to, on a disk with room for the project.",
		Cause: cause,
	}
}

func errNotResolved(dir, output string) error {
	return &diag.Error{
		Msg:  "pkl project resolve failed:\n" + fsx.TrimASCIISpace(output),
		File: fileOf(dir, "PklProject"),
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

func errAnotherDrive(schema string) error {
	return &diag.Error{
		Msg: "--link needs the project on the same drive as this Moonwell checkout.",
		Hint: "Create the project on " + filepath.VolumeName(schema) + `\` +
			" (Pkl cannot load a local dependency from another drive), or use the published package with a plain " +
			"`init`.",
	}
}
