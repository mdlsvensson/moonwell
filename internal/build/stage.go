package build

import (
	"errors"
	"strconv"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/env"
	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/manifest"
)

// This file holds the stage: the folder a planned map is written to, for the game to load and the user to read.

// stage writes the planned map to dist/stage/<map.folder>, in place of what a build left there, and says what
// was written into it of the project. It returns where the map is staged.
func stage(e *env.Env, p *manifest.Project, plan *Result) (place, error) {
	at, err := stagePlace(p)
	if err != nil {
		return place{}, err
	}
	if err := plan.Map.StageTo(at.file); err != nil {
		return place{}, stagingFailure(err, at)
	}
	sayStaged(e.Log, plan)
	return at, nil
}

// stagePlace is where the project's map is staged: dist/stage/<map.folder>, a folder named as the source map
// is, since the game loads a map that is a folder by its .w3x name.
//
// One corner: a map folder below lua/, such as lua/one.w3x, has its stage inside the folder of the compile's
// cache, dist/stage/lua. Neither touches what the other wrote, because a map's folder ends in .w3x, as the
// schema has it, and nothing of the cache is named so. The cache holds .hashes.json, .globals.json and one .lua
// for each module, below the folders of the module's name, which hold no dot, and for a library's module below
// .libraries and the library's key, which is letters, digits, "_" and "-". And each clears only its own: the
// compile removes the files it wrote, each by its name, and staging replaces the stage's own folder.
func stagePlace(p *manifest.Project) (place, error) {
	folder, err := mapFolder(p)
	if err != nil {
		return place{}, err
	}
	return placeOf(p.Root, stageDir+"/"+folder)
}

// stagingFailure is a failure of StageTo as a command reports it. A failure of the system, which is one that has
// a cause, is named from the project folder: by the stage, and by the file of it that could not be written. A
// refusal of the plan stays as it is: a stage that would replace the source map, and a planner's bug.
func stagingFailure(err error, at place) error {
	var failure *diag.Error
	if !errors.As(err, &failure) || failure.Cause == nil {
		return err
	}
	return errNotStaged(at.label, at.labelOf(failure.File), failure.Cause)
}

// sayStaged logs what was written into the stage of the project: its objects, its settings and its assets, each
// in a line where there are any, and the lines that say which of a library's files the map's own replace. The
// lines come after the stage is written, so a build that fails before that logs none of them.
func sayStaged(log *env.Logger, plan *Result) {
	if count := len(plan.Objects.Objects); count > 0 {
		log.Info("Added " + strconv.Itoa(count) + " custom object(s) to " + strconv.Itoa(len(plan.Objects.Changes)) +
			" file(s).")
	}
	if count := len(plan.Settings); count > 0 {
		log.Info("Applied map settings to " + strconv.Itoa(count) + " internal file(s).")
	}
	for _, line := range plan.Replaced {
		log.Info(line)
	}
	if count := len(plan.Assets.Assets); count > 0 {
		log.Info("Imported " + strconv.Itoa(count) + " asset(s).")
	}
}

// ---- errors ----

// stagingHint ends a failure to write the stage.
const stagingHint = "Close Warcraft III or World Editor if they have dist/stage open, then retry."

func errNotStaged(stage, file string, cause error) error {
	return &diag.Error{
		Msg:   "Staging the map into " + stage + " failed: " + fsx.Reason(cause),
		File:  file,
		Hint:  stagingHint,
		Cause: cause,
	}
}
