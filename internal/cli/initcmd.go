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

// runInit is `moonwell init <dir> [--link]`: it makes a project in a folder that is new or empty. With --link
// the project uses the Pkl package of the Moonwell checkout the command runs in, in place of the published one.
func runInit(ctx context.Context, e *env.Env, c call) error {
	arguments := c.said.arguments()
	// An empty name is no folder: read against the working folder, it would be the working folder itself.
	if len(arguments) == 0 || arguments[0] == "" {
		return errInitNeedsAFolder()
	}
	schema := "" // the published package
	if c.said.link {
		checkout, err := checkoutAbove(e.Root)
		if err != nil {
			return err
		}
		schema = filepath.Join(checkout, "schema")
	}
	return createProject(ctx, e, arguments[0], schema)
}

// createProject makes a project in dir, a folder as the command line names it, from e.Root: the template's
// files, a PklProject and a moonwell.local.pkl, with the project's Pkl dependencies resolved. schema is the
// folder of the Pkl package the project is linked to, a checkout's schema/; "" is the published package.
//
// Nothing is written before the folder is known to be new or empty and Pkl is found. A project that could not be
// written whole, or resolved, is undone, as undoInit says.
func createProject(ctx context.Context, e *env.Env, dir, schema string) error {
	target := fsx.Resolve(e.Root, dir)
	existed, err := newOrEmpty(target, dir)
	if err != nil {
		return err
	}
	local, err := linkTo(target, schema) // how the PklProject names the schema; "" for the published package
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

// newOrEmpty refuses a target that is no place for a new project: a file, and a folder that holds something.
// existed says that the folder is there, and empty. dir is the target as messages name it.
func newOrEmpty(target, dir string) (existed bool, err error) {
	info, err := os.Stat(target)
	switch {
	// Nothing is below a file either: one system says of such a place that it is not there, and another that
	// what is above it is no folder.
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

// refuseLinkToNothing refuses a target that the system says is not there though its name is taken: a link that
// leads to nothing. It is no folder, and what undoes a failed init would remove it as a folder init made.
func refuseLinkToNothing(target, dir string) error {
	if info, err := fsx.Lstat(target); err == nil && info != nil {
		return errNotAFolder(dir)
	}
	return nil
}

// writeProject writes the files of a new project into target: the template's, the PklProject, which names the
// package at local or else the published one, and moonwell.local.pkl. dir is the target as messages name it.
func writeProject(target, dir, local string) error {
	files, err := moonwell.TemplateFiles()
	if err != nil {
		// A plain error: the template is part of the program, so one that cannot be read is a mistake in Moonwell
		// and nothing the user can put right.
		return err
	}
	files = append(files,
		moonwell.TemplateFile{Path: "PklProject", Data: []byte(manifest.PklProject(moonwell.Version, local))},
		moonwell.TemplateFile{Path: manifest.LocalFile, Data: []byte(manifest.LocalPkl())},
	)
	for _, file := range files {
		if err := writeProjectFile(target, dir, file); err != nil {
			return err
		}
	}
	return nil
}

// writeProjectFile writes one file of a new project, with the folders it is in.
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

// fileOf is a file of the new project as a message names it: the folder init was given and then the file's path
// in the project, name, with "/" on every system.
func fileOf(dir, name string) string {
	return fsx.ToPosix(filepath.Join(dir, filepath.FromSlash(name)))
}

// resolve has pkl resolve the dependencies of the new project in target, which writes its PklProject.deps.json:
// a manifest is evaluated against the package that file names.
func resolve(ctx context.Context, e *env.Env, pkl, target, dir string) error {
	result, err := e.Run(ctx, pkl, []string{"project", "resolve"}, env.RunOptions{Dir: target})
	if err != nil {
		return err
	}
	if result.Code != 0 {
		return errNotResolved(dir, cmp.Or(result.Stderr, result.Stdout))
	}
	return nil
}

// undoInit removes the project of an init that failed. When init made the folder, the folder goes, with all that
// is in it. When the folder was there and empty, everything in it goes and the folder stays: a file that another
// program put there since init looked goes too. The folders init made above the target stay.
//
// It does what it can: the failure that led here matters more than one of its own.
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

// moduleLine is the line of a go.mod that names Moonwell's module.
var moduleLine = regexp.MustCompile(`(?m)^module\s+github\.com/mdlsvensson/moonwell\s*$`)

// checkoutAbove is the Moonwell checkout that start is in: the folder at or above it whose go.mod names this
// module.
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

// linkTo is how the PklProject of a project in target names the package it uses: the way to schema for a
// project that is linked to one, and "" for one that uses the published package.
func linkTo(target, schema string) (string, error) {
	if schema == "" {
		return "", nil
	}
	return linkPath(target, schema)
}

// linkPath is how a linked project in target refers to a checkout's schema folder: always by a relative path,
// with "/". On Windows there is none across drives, and Pkl cannot load a local dependency from another drive,
// since PklProject.deps.json cannot record the path: that is refused.
func linkPath(target, schema string) (string, error) {
	inside, err := filepath.Rel(target, schema)
	if err != nil || filepath.IsAbs(inside) {
		return "", errAnotherDrive(schema)
	}
	return fsx.ToPosix(inside), nil
}

// ---- errors ----

// chooseEmpty is the hint of every refusal of the folder init is given.
const chooseEmpty = "Choose a new or empty directory."

// errInitNeedsAFolder refuses an init without its argument, as the line is read.
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

// errNotWritten is the failure to write a file of the new project; file is named from the folder init was given.
func errNotWritten(file string, cause error) error {
	return &diag.Error{
		Msg:   "Writing " + file + " failed: " + fsx.Reason(cause),
		File:  file,
		Hint:  "Choose a directory that you may write to, on a disk with room for the project.",
		Cause: cause,
	}
}

// errNotResolved shows what pkl printed when it failed to resolve the project's dependencies. The hint names the
// one cause that is not the project's: a project that is not linked to a checkout has its package fetched.
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
