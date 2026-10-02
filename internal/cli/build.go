package cli

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/mpq"
	"github.com/mdlsvensson/moonwell/internal/pipeline"
	"github.com/mdlsvensson/moonwell/internal/project"
)

// relative is path as a POSIX path from root, for messages.
func relative(root, path string) string {
	inside, err := filepath.Rel(root, path)
	if err != nil {
		return fsx.ToPosix(path)
	}
	return fsx.ToPosix(inside)
}

// Build builds <build.folder>/<map.folder> and returns the archive's path. A failed build leaves no archive behind.
func Build(ctx context.Context, env *pipeline.Env, options pipeline.StageOptions) (string, error) {
	// Loading only reads; doing it before taking the lock creates nothing outside a project.
	p, err := project.Load(ctx, env.Root, env.Run)
	if err != nil {
		return "", err
	}
	release, err := pipeline.AcquireLock(filepath.Join(env.Root, "dist"))
	if err != nil {
		return "", err
	}
	defer release()
	output, err := ArchivePath(env.Root, p)
	if err != nil {
		return "", err
	}
	if err := fsx.RemoveFile(output); err != nil {
		return "", err
	}
	if err := build(ctx, env, p, options, output); err != nil {
		// The removal's own failure matters less than the build's.
		fsx.RemoveFile(output)
		return "", err
	}
	return output, nil
}

func build(ctx context.Context, env *pipeline.Env, p *project.Project, options pipeline.StageOptions, output string) error {
	mapDir, modules, err := pipeline.PrepareStage(ctx, env, p, options)
	if err != nil {
		return err
	}
	env.Log.Info("Packing archive...")
	archive, err := mpq.PackMap(mapDir, strings.TrimSuffix(filepath.Base(p.Map.Folder), ".w3x"))
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(output), 0o777); err != nil {
		return err
	}
	if err := os.WriteFile(output, archive, 0o666); err != nil {
		return err
	}
	env.Log.Info("Built " + relative(env.Root, output) + " (" + strconv.Itoa(len(modules)) + " module(s)).")
	return nil
}

// ArchivePath is where the archive goes. It must lie inside the project and must not be a folder, since a build
// deletes it.
func ArchivePath(root string, p *project.Project) (string, error) {
	base, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	output := fsx.Resolve(fsx.Resolve(base, p.Build.Folder), p.Map.Folder)
	inside, err := filepath.Rel(base, output)
	if err != nil || inside == "." || inside == ".." || strings.HasPrefix(inside, ".."+string(filepath.Separator)) || filepath.IsAbs(inside) {
		return "", &diag.Error{
			Msg:  "The build output " + output + " is outside the project.",
			File: p.Manifest,
			Hint: "Set build.folder to a folder inside the project, such as dist/bin.",
		}
	}
	if fsx.IsDir(output) {
		return "", &diag.Error{
			Msg:  "The build output " + fsx.ToPosix(inside) + " is a directory; refusing to replace it.",
			File: p.Manifest,
			Hint: "Set build.folder to a folder that only holds build output, such as dist/bin.",
		}
	}
	return output, nil
}
