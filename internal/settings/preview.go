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

func loadPreview(root, preview, manifestName string) (*picture.Picture, error) {
	path, err := previewPath(preview, manifestName)
	if err != nil {
		return nil, err
	}
	data, err := readPreview(root, path, manifestName)
	if err != nil {
		return nil, err
	}
	return picture.Read(data, path)
}

func previewPath(preview, manifestName string) (string, error) {
	path, inside := fsx.CleanRelPath(preview)
	switch {
	case !inside:
		return "", errOutsideProject(manifestName, preview)
	case strings.HasPrefix(strings.ToLower(path), "assets/"):
		return "", errUnderAssets(manifestName, path)
	}
	return path, nil
}

func readPreview(root, path, manifestName string) ([]byte, error) {
	fullPath, err := fsx.SafeJoin(root, path)
	if err != nil {
		return nil, wrapPreviewError(err, path, manifestName)
	}
	info, err := fsx.Lstat(fullPath)
	switch {
	case err != nil:
		return nil, wrapPreviewError(err, path, manifestName)
	case info == nil:
		return nil, errNoSuchFile(manifestName, path)
	case !info.Mode().IsRegular():
		return nil, errNotAFile(manifestName, path)
	}
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, errUnreadable(path, err)
	}
	return data, nil
}

func wrapPreviewError(err error, path, manifestName string) error {
	var diagErr *diag.Error
	switch {
	case errors.As(err, &diagErr):
		return err
	case errors.Is(err, syscall.ENOTDIR):
		return errNoSuchFile(manifestName, path)
	}
	return errUnreachable(path, err)
}

func errOutsideProject(manifestName, preview string) error {
	return &diag.Error{
		Msg:  `settings.info.preview must be a path inside the project, not "` + preview + `".`,
		File: manifestName,
		Hint: `Name a picture in the project folder, such as "preview.tga" beside moonwell.pkl.`,
	}
}

func errUnderAssets(manifestName, path string) error {
	return &diag.Error{
		Msg:  "settings.info.preview names a file under assets/: " + path,
		File: manifestName,
		Hint: "Keep the picture outside assets/, for example beside moonwell.pkl: every file under assets/ is also " +
			"imported into the map under its own name.",
	}
}

const fromProjectFolder = "The path starts at the project folder, where moonwell.pkl is."

func errNoSuchFile(manifestName, path string) error {
	return &diag.Error{
		Msg:  "settings.info.preview names a file that does not exist: " + path,
		File: manifestName,
		Hint: fromProjectFolder,
	}
}

func errNotAFile(manifestName, path string) error {
	return &diag.Error{
		Msg:  "settings.info.preview does not name a file: " + path,
		File: manifestName,
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
