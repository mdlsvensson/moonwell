package settings

import (
	"errors"
	"os"
	"strings"
	"syscall"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/war3/picture"
)

func loadPreview(root, preview, manifestFile string) (*picture.Picture, error) {
	path, err := previewPath(preview, manifestFile)
	if err != nil {
		return nil, err
	}
	data, err := readPreview(root, path, manifestFile)
	if err != nil {
		return nil, err
	}
	return picture.Read(data, path)
}

func previewPath(preview, manifestFile string) (string, error) {
	path, inside := fsx.RelPath(preview)
	switch {
	case !inside:
		return "", errOutsideProject(manifestFile, preview)
	case strings.HasPrefix(strings.ToLower(path), "assets/"):
		return "", errUnderAssets(manifestFile, path)
	}
	return path, nil
}

func readPreview(root, path, manifestFile string) ([]byte, error) {
	file, err := fsx.SafeJoin(root, path)
	if err != nil {
		return nil, unreached(err, path, manifestFile)
	}
	info, err := fsx.Lstat(file)
	switch {
	case err != nil:
		return nil, unreached(err, path, manifestFile)
	case info == nil:
		return nil, errNoSuchFile(manifestFile, path)
	case !info.Mode().IsRegular():
		return nil, errNotAFile(manifestFile, path)
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, errUnreadable(path, err)
	}
	return data, nil
}

func unreached(err error, path, manifestFile string) error {
	var expected *diag.Error
	switch {
	case errors.As(err, &expected):
		return err
	case errors.Is(err, syscall.ENOTDIR):
		return errNoSuchFile(manifestFile, path)
	}
	return errUnreachable(path, err)
}

func errOutsideProject(manifestFile, preview string) error {
	return &diag.Error{
		Msg:  `settings.info.preview must be a path inside the project, not "` + preview + `".`,
		File: manifestFile,
		Hint: `Name a picture in the project folder, such as "preview.tga" beside moonwell.pkl.`,
	}
}

func errUnderAssets(manifestFile, path string) error {
	return &diag.Error{
		Msg:  "settings.info.preview names a file under assets/: " + path,
		File: manifestFile,
		Hint: "Keep the picture outside assets/, for example beside moonwell.pkl: every file under assets/ is also " +
			"imported into the map under its own name.",
	}
}

const fromProjectFolder = "The path starts at the project folder, where moonwell.pkl is."

func errNoSuchFile(manifestFile, path string) error {
	return &diag.Error{
		Msg:  "settings.info.preview names a file that does not exist: " + path,
		File: manifestFile,
		Hint: fromProjectFolder,
	}
}

func errNotAFile(manifestFile, path string) error {
	return &diag.Error{
		Msg:  "settings.info.preview does not name a file: " + path,
		File: manifestFile,
		Hint: fromProjectFolder,
	}
}

func errUnreadable(path string, cause error) error {
	return &diag.Error{
		Msg:   "Reading the preview picture failed: " + fsx.Reason(cause),
		File:  path,
		Cause: cause,
		Hint:  "Make sure no other program has the picture locked.",
	}
}

func errUnreachable(path string, cause error) error {
	return &diag.Error{
		Msg:   "Reading the preview picture failed: " + fsx.Reason(cause),
		File:  path,
		Cause: cause,
		Hint:  "Make sure the picture and every folder on the way to it can be read.",
	}
}
