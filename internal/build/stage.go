package build

import (
	"errors"
	"strconv"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/env"
	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/manifest"
)

func stage(e *env.Env, project *manifest.Project, plan *Result) (outputFile, error) {
	staged, err := stageOutputFile(project)
	if err != nil {
		return outputFile{}, err
	}
	if err := plan.Map.StageTo(staged.fullPath); err != nil {
		return outputFile{}, wrapStageError(err, staged)
	}
	logStaged(e.Log, plan)
	return staged, nil
}

func stageOutputFile(project *manifest.Project) (outputFile, error) {
	mapDir, err := sourceMapDir(project)
	if err != nil {
		return outputFile{}, err
	}
	return newOutputFile(project.Root, stageDir+"/"+mapDir)
}

func wrapStageError(err error, staged outputFile) error {
	var diagErr *diag.Error
	if !errors.As(err, &diagErr) || diagErr.Cause == nil {
		return err
	}
	return errNotStaged(staged.displayPath, staged.displayPathOf(diagErr.File), diagErr.Cause)
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

func errNotStaged(stageDisplayPath, displayPath string, cause error) error {
	return &diag.Error{
		Msg:   "Staging the map into " + stageDisplayPath + " failed: " + fsx.Reason(cause),
		File:  displayPath,
		Hint:  stagingHint,
		Cause: cause,
	}
}
