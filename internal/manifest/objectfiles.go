package manifest

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/env"
	"github.com/mdlsvensson/moonwell/internal/fsx"
)

const (
	ObjectsDir    = "objects"
	ObjectsModule = ".moonwell/objects.pkl"
)

const objectsModuleText = `import "@moonwell/Objects.pkl"

output {
  value = Objects.merge(import*("../objects/**.pkl"))
}
`

func HasObjectFiles(root string) (bool, error) {
	files, err := fsx.ListFiles(filepath.Join(root, ObjectsDir))
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, errObjectsDirUnreadable(err)
	}
	for _, file := range files {
		if strings.HasSuffix(file, ".pkl") {
			return true, nil
		}
	}
	return false, nil
}

func EvaluateObjects(ctx context.Context, e *env.Env, pkl string) (Objects, error) {
	if err := checkResolvedPackage(e.Root); err != nil {
		return Objects{}, err
	}
	if err := writeObjectsModule(e.Root); err != nil {
		return Objects{}, err
	}
	args := []string{"eval", "--format", "json", "--project-dir", ".", ObjectsModule}
	result, err := e.Run(ctx, pkl, args, env.RunOptions{Dir: e.Root})
	if err != nil {
		return Objects{}, err
	}
	if result.ExitCode != 0 {
		return Objects{}, errObjectsNotEvaluated(cmp.Or(result.Stderr, result.Stdout))
	}
	var objects Objects
	if err := json.Unmarshal([]byte(result.Stdout), &objects); err != nil {
		return Objects{}, errObjectsNotJSON(result.Stdout, err)
	}
	objects.pointSourcesAtProject()
	return objects, nil
}

func writeObjectsModule(root string) error {
	fullPath, err := fsx.SafeJoinNoSymlinks(root, ObjectsModule)
	if err != nil {
		return err
	}
	if _, err := fsx.WriteIfChanged(fullPath, objectsModuleText); err != nil {
		return errObjectsModuleNotWritten(err)
	}
	return nil
}

func (o *Objects) pointSourcesAtProject() {
	for _, category := range Categories {
		objects := o.pointerTo(category)
		for key, object := range objects.All() {
			object.Source = strings.TrimPrefix(object.Source, "../")
			objects.Set(key, object)
		}
	}
}

func errObjectsDirUnreadable(cause error) error {
	return &diag.Error{
		Msg:   "Reading the folder " + ObjectsDir + " failed: " + fsx.Reason(cause),
		File:  ObjectsDir,
		Hint:  "Check that it is a folder this user may read.",
		Cause: cause,
	}
}

func errObjectsNotEvaluated(output string) error {
	return &diag.Error{Msg: "Evaluating the object files failed:\n" + fsx.TrimASCIISpace(output), File: ObjectsDir}
}

func errObjectsNotJSON(output string, cause error) error {
	return &diag.Error{
		Msg:  "pkl eval printed output that is not the objects as JSON:\n" + truncateRunes(fsx.TrimASCIISpace(output), 500),
		File: ObjectsDir,
		Hint: "Every file under " + ObjectsDir + "/ must amend \"@moonwell/ObjectFile.pkl\", and the moonwell package in " +
			"PklProject must be of this program's version.",
		Cause: cause,
	}
}

func errObjectsModuleNotWritten(cause error) error {
	return &diag.Error{
		Msg:   "Writing " + ObjectsModule + " failed: " + fsx.Reason(cause),
		File:  ObjectsModule,
		Hint:  "Make sure that the project folder is one you may write to and that its disk has room.",
		Cause: cause,
	}
}
