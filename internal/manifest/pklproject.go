package manifest

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	moonwell "github.com/mdlsvensson/moonwell"
	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/fsx"
)

const (
	pklProjectFile = "PklProject"
	depsFile       = "PklProject.deps.json"
)

const packageBaseURI = "package://pkg.pkl-lang.org/github.com/mdlsvensson/moonwell/moonwell"

func PklProjectText(version, schemaDir string) string {
	line := `  ["moonwell"] { uri = "` + packageBaseURI + "@" + version + `" }`
	if schemaDir != "" {
		line = `  ["moonwell"] = import("` + schemaDir + `/PklProject")`
	}
	return "amends \"pkl:Project\"\n\ndependencies {\n" + line + "\n}\n"
}

func checkResolvedPackage(root string) error {
	deps, err := os.ReadFile(filepath.Join(root, depsFile))
	if errors.Is(err, fs.ErrNotExist) {
		return errDepsMissing()
	}
	if err != nil {
		return errDepsUnreadable(err)
	}
	version, err := readPackageVersion(deps)
	if err != nil {
		return err
	}
	return checkPackageVersion(version, moonwell.Version)
}

func errDepsMissing() error {
	return &diag.Error{
		Msg:  "PklProject.deps.json is missing.",
		File: pklProjectFile,
		Hint: "Run `pkl project resolve` in the project folder.",
	}
}

func errDepsUnreadable(cause error) error {
	return &diag.Error{
		Msg:   "Reading PklProject.deps.json failed: " + fsx.Reason(cause),
		File:  depsFile,
		Hint:  "Check that it is a file this user may read, or write it again with `pkl project resolve`.",
		Cause: cause,
	}
}
