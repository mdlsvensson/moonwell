package settings

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/mdlsvensson/moonwell/next/internal/fsx"
	"github.com/mdlsvensson/moonwell/next/internal/testkit"
)

func opaque() *byte {
	alpha := byte(255)
	return &alpha
}

// plainTGA is a picture the game shows, as a file loadPreview takes.
func plainTGA() []byte { return testkit.TGA(testkit.NewPixels(256), testkit.TGAOptions{}) }

func TestAPreviewOfEachKindIsReadFromItsPathInTheProject(t *testing.T) {
	picture := testkit.NewPixels(256)
	asTGA := testkit.TGA(picture, testkit.TGAOptions{Alpha: opaque()})
	blp := testkit.BLP(256, 1)
	tests := []struct {
		name, file, preview string
		data                []byte
		extension           string
		want                []byte
	}{
		{"a BLP beside the manifest goes in as it is", "preview.blp", "preview.blp", blp, "blp", blp},
		{"a TGA in a folder, named in capitals, is written again", "art/Preview.TGA", "art/Preview.TGA",
			testkit.TGA(picture, testkit.TGAOptions{RLE: true, Depth: 24, FromTop: true}), "tga", asTGA},
		{"a PNG goes in as the TGA of its picture", "art/Preview.PNG", "art/Preview.PNG", testkit.PNG(picture, "rgba"), "tga", asTGA},
		{"a path written with backslashes", "art/deep/preview.tga", `art\deep\preview.tga`, plainTGA(), "tga", asTGA},
		{"a folder named like assets is not assets", "assets2/preview.blp", "assets2/preview.blp", blp, "blp", blp},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			testkit.WriteFile(t, root, tt.file, tt.data)
			got, err := loadPreview(root, tt.preview, manifestName)
			if err != nil {
				t.Fatal(err)
			}
			if got.Extension != tt.extension || !bytes.Equal(got.Bytes, tt.want) {
				t.Errorf("the picture is a %s of %d bytes, want a %s of %d bytes", got.Extension, len(got.Bytes), tt.extension, len(tt.want))
			}
		})
	}
}

func TestAPreviewSettingThatNamesNoUsablePictureIsRefused(t *testing.T) {
	root := t.TempDir()
	testkit.WriteFile(t, root, "preview.tga", plainTGA())
	testkit.WriteFile(t, root, "assets/preview.tga", plainTGA())
	testkit.WriteFile(t, root, "preview.jpg", plainTGA())
	testkit.WriteFile(t, root, "preview.png", plainTGA())
	testkit.WriteFile(t, root, "small.tga", plainTGA()[:100])
	if err := os.Mkdir(filepath.Join(root, "folder.tga"), 0o777); err != nil {
		t.Fatal(err)
	}
	inside := func(preview string) string {
		return `settings.info.preview must be a path inside the project, not "` + preview + `".`
	}
	tests := []struct{ name, preview, words, file string }{
		{"a file that is not there", "missing.tga", "settings.info.preview names a file that does not exist: missing.tga", manifestName},
		{"a file in a folder that is not there", "art/preview.tga", "names a file that does not exist: art/preview.tga", manifestName},
		{"a folder", "folder.tga", "settings.info.preview does not name a file: folder.tga", manifestName},
		{"a file under assets", "assets/preview.tga", "settings.info.preview names a file under assets/: assets/preview.tga", manifestName},
		{"a file under assets, in another spelling", `Assets\preview.tga`, "names a file under assets/: Assets/preview.tga", manifestName},
		{"a file under assets that is not there", "assets/missing.tga", "names a file under assets/: assets/missing.tga", manifestName},
		{"a path that leaves the project", "../preview.tga", inside("../preview.tga"), manifestName},
		{"a path from the root of the disk", "/preview.tga", inside("/preview.tga"), manifestName},
		{"a path with a drive", `C:\preview.tga`, inside(`C:\preview.tga`), manifestName},
		{"a path with an empty name", "art//preview.tga", inside("art//preview.tga"), manifestName},
		{"a file of another format", "preview.jpg", "The preview picture must be a .tga, a .blp or a .png file.", "preview.jpg"},
		{"a file that is not what its name says", "preview.png", "The preview picture is not a PNG file", "preview.png"},
		{"a picture cut short", "small.tga", "The preview picture is cut short", "small.tga"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := loadPreview(root, tt.preview, manifestName)
			failure := asError(t, err, tt.preview)
			if !strings.Contains(failure.Msg, tt.words) || failure.File != tt.file || failure.Hint == "" {
				t.Errorf("error = %+v, want %q naming %s", failure, tt.words, tt.file)
			}
		})
	}
}

func TestAPreviewThatCannotBeReadIsRefusedByItsPath(t *testing.T) {
	root := t.TempDir()
	makeUnreadable(t, testkit.WriteFile(t, root, "art/preview.tga", plainTGA()))
	_, err := loadPreview(root, "art/preview.tga", manifestName)
	failure := asError(t, err, "a picture that cannot be read")
	if !strings.Contains(failure.Msg, "Reading the preview picture failed: ") || failure.File != "art/preview.tga" ||
		failure.Hint == "" || failure.Cause == nil {
		t.Errorf("error = %+v", failure)
	}
}

// A file where the path needs a folder: the picture does not exist, whatever the operating system calls that.
func TestAPreviewPathThroughAFileNamesAFileThatDoesNotExist(t *testing.T) {
	root := t.TempDir()
	testkit.WriteFile(t, root, "preview.tga", plainTGA())
	_, err := loadPreview(root, "preview.tga/inner.tga", manifestName)
	failure := asError(t, err, "a path through a file")
	if !strings.Contains(failure.Msg, "names a file that does not exist: preview.tga/inner.tga") ||
		failure.File != manifestName || failure.Hint == "" {
		t.Errorf("error = %+v", failure)
	}
}

// What the operating system says on the way to the picture, given here as it says it on each system: a file
// system gives only its own.
func TestAFailureOnTheWayToThePreviewIsToldAsWhatTheUserCanFix(t *testing.T) {
	const path = "art/preview.tga"
	failed := func(reason error) error {
		return &fs.PathError{Op: "lstat", Path: filepath.Join("project", "art", "preview.tga"), Err: reason}
	}
	t.Run("a file where the path needs a folder", func(t *testing.T) {
		failure := asError(t, unreached(failed(syscall.ENOTDIR), path, manifestName), "not a directory")
		if !strings.Contains(failure.Msg, "names a file that does not exist: "+path) || failure.File != manifestName ||
			failure.Hint == "" {
			t.Errorf("error = %+v", failure)
		}
	})
	t.Run("a folder that may not be entered", func(t *testing.T) {
		cause := failed(syscall.EACCES)
		failure := asError(t, unreached(cause, path, manifestName), "permission denied")
		if !strings.Contains(failure.Msg, "Reading the preview picture failed: ") || failure.File != path ||
			failure.Cause != cause {
			t.Errorf("error = %+v", failure)
		}
		if !strings.Contains(failure.Hint, "folder") || strings.Contains(failure.Hint, "locked") {
			t.Errorf("the hint is %q, want one that fits a folder and claims no lock", failure.Hint)
		}
	})
	t.Run("a link", func(t *testing.T) {
		link := fsx.LinkError(filepath.Join("project", "art"))
		if got := unreached(link, path, manifestName); got != link {
			t.Errorf("error = %v, want the link's own", got)
		}
	})
}

func TestAPreviewBehindALinkIsRefused(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	testkit.WriteFile(t, outside, "preview.tga", plainTGA())
	testkit.LinkDir(t, outside, filepath.Join(root, "art"))
	_, err := loadPreview(root, "art/preview.tga", manifestName)
	if failure := asError(t, err, "a picture behind a link"); !strings.Contains(failure.Msg, "Symlinks are not supported") {
		t.Errorf("error = %+v", failure)
	}
}
