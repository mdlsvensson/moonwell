package build

import (
	"errors"
	"strconv"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/env"
	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/manifest"
)

func stage(e *env.Env, p *manifest.Project, plan *Result) (outputFile, error) {
	at, err := stageOutputFile(p)
	if err != nil {
		return outputFile{}, err
	}
	if err := plan.Map.StageTo(at.fullPath); err != nil {
		return outputFile{}, wrapStageError(err, at)
	}
	logStaged(e.Log, plan)
	return at, nil
}

func stageOutputFile(p *manifest.Project) (outputFile, error) {
	folder, err := sourceMapDir(p)
	if err != nil {
		return outputFile{}, err
	}
	return newOutputFile(p.Root, stageDir+"/"+folder)
}

func wrapStageError(err error, at outputFile) error {
	var failure *diag.Error
	if !errors.As(err, &failure) || failure.Cause == nil {
		return err
	}
	return errNotStaged(at.displayPath, at.displayPathOf(failure.File), failure.Cause)
}

func logStaged(log *env.Logger, plan *Result) {
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

const stagingHint = "Close Warcraft III or World Editor if they have dist/stage open, then retry."

func errNotStaged(stage, file string, cause error) error {
	return &diag.Error{
		Msg:   "Staging the map into " + stage + " failed: " + fsx.Reason(cause),
		File:  file,
		Hint:  stagingHint,
		Cause: cause,
	}
}
