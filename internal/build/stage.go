package build

import (
	"errors"
	"strconv"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/env"
	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/manifest"
)

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

func stagePlace(p *manifest.Project) (place, error) {
	folder, err := mapFolder(p)
	if err != nil {
		return place{}, err
	}
	return placeOf(p.Root, stageDir+"/"+folder)
}

func stagingFailure(err error, at place) error {
	var failure *diag.Error
	if !errors.As(err, &failure) || failure.Cause == nil {
		return err
	}
	return errNotStaged(at.label, at.labelOf(failure.File), failure.Cause)
}

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

const stagingHint = "Close Warcraft III or World Editor if they have dist/stage open, then retry."

func errNotStaged(stage, file string, cause error) error {
	return &diag.Error{
		Msg:   "Staging the map into " + stage + " failed: " + fsx.Reason(cause),
		File:  file,
		Hint:  stagingHint,
		Cause: cause,
	}
}
