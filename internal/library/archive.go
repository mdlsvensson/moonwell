package library

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"slices"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/env"
	"github.com/mdlsvensson/moonwell/internal/fsx"
)

// file is one file of a library: its path inside the library, with "/", and its bytes.
type file struct {
	name string
	data []byte
}

// downloadTag downloads a tag of the library key from GitHub: the tag's commit and its files, without the
// archive's top folder, every path checked to stay inside the library's folder. Failures name manifestFile.
func downloadTag(
	ctx context.Context, fetch env.FetchFunc, key, github, tag, manifestFile string,
) (commit string, files []file, err error) {
	address := archiveURL(github, tag)
	body, err := fetchTag(ctx, fetch, address, key, github, tag, manifestFile)
	if err != nil {
		return "", nil, err
	}
	commit, files, err = readArchive(body)
	if err != nil {
		return "", nil, errNotATagArchive(key, manifestFile, address, err)
	}
	for _, f := range files {
		if !insideLibrary(f.name) {
			return "", nil, errUnsafePath(key, manifestFile, address, f.name)
		}
	}
	return commit, files, nil
}

// fetchTag asks address for the archive of a tag, and returns what a status from 200 to 299 came with.
func fetchTag(ctx context.Context, fetch env.FetchFunc, address, key, github, tag, manifestFile string) ([]byte, error) {
	status, body, err := fetch(ctx, address)
	switch {
	case err != nil:
		return nil, errDownloadFailed(key, github, manifestFile, err)
	case status == 404:
		return nil, errNoSuchTag(key, github, tag, manifestFile)
	case status < 200 || status > 299:
		return nil, errStatus(key, status, manifestFile)
	}
	return body, nil
}

// archiveURL is the address of the zip archive of a GitHub tag. Each part of the tag between two "/" is written as
// a segment of a path is, and the "/" stays between them.
func archiveURL(github, tag string) string {
	segments := strings.Split(tag, "/")
	for i, segment := range segments {
		segments[i] = url.PathEscape(segment)
	}
	return "https://codeload.github.com/" + github + "/zip/refs/tags/" + strings.Join(segments, "/")
}

// readArchive reads a GitHub tag archive: the commit, which the archive's comment holds, and the files without
// their single top folder, whose name GitHub makes from the repository and the tag. Folder entries are skipped,
// so every name is a file's.
//
// Its failures say what is wrong with the bytes and nothing else: downloadTag says whose bytes they are.
func readArchive(data []byte) (commit string, files []file, err error) {
	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	// A name that leaves the archive's folder is no failure of reading: the reader that comes with it holds
	// every entry, and downloadTag refuses the name.
	if err != nil && !errors.Is(err, zip.ErrInsecurePath) {
		return "", nil, errInvalidZip(err)
	}
	if commit, err = commitOf(archive.Comment); err != nil {
		return "", nil, err
	}
	if files, err = filesBelowTheTop(archive.File); err != nil {
		return "", nil, err
	}
	return commit, files, nil
}

// commitOf is the commit an archive's comment holds: forty hexadecimal digits in lower case, with nothing but
// ASCII white space around them.
func commitOf(comment string) (string, error) {
	commit := fsx.TrimASCIISpace(comment)
	if len(commit) != 40 || strings.ContainsFunc(commit, notALowerHexDigit) {
		return "", errNoCommit()
	}
	return commit, nil
}

func notALowerHexDigit(r rune) bool { return !strings.ContainsRune("0123456789abcdef", r) }

// filesBelowTheTop reads the file entries of an archive in their order, each under its path below the one top
// folder that all of them are in. A path that comes twice keeps its first place and its last bytes.
func filesBelowTheTop(entries []*zip.File) ([]file, error) {
	var files []file
	place := map[string]int{} // where in files each path is
	top, hasTop := "", false
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name, "/") {
			continue
		}
		first, path, found := strings.Cut(entry.Name, "/")
		if !found || (hasTop && first != top) {
			return nil, errNoSingleTop()
		}
		top, hasTop = first, true
		data, err := contentOf(entry)
		if err != nil {
			return nil, err
		}
		if at, held := place[path]; held {
			files[at].data = data
			continue
		}
		place[path] = len(files)
		files = append(files, file{path, data})
	}
	return files, nil
}

// contentOf is the bytes of an entry, unpacked and checked against the entry's checksum.
func contentOf(entry *zip.File) ([]byte, error) {
	reader, err := entry.Open()
	if err != nil {
		return nil, errEntryUnreadable(entry.Name, err)
	}
	defer reader.Close()
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, errEntryUnreadable(entry.Name, err)
	}
	return data, nil
}

// filesHash is "sha256:" and the SHA-256 of `<path>\n<sha256 of its bytes>\n` for each file, in the byte order of
// the paths.
func filesHash(files []file) string {
	sorted := slices.SortedFunc(slices.Values(files), func(a, b file) int { return strings.Compare(a.name, b.name) })
	var lines strings.Builder
	for _, f := range sorted {
		lines.WriteString(f.name + "\n" + fsx.SHA256Hex(f.data) + "\n")
	}
	return "sha256:" + fsx.SHA256Hex([]byte(lines.String()))
}

// reasonOf is the text of a failure inside another message: what an expected failure says, or the system's
// reason without the operation and the path.
func reasonOf(err error) string {
	var failure *diag.Error
	if errors.As(err, &failure) {
		return failure.Msg
	}
	return fsx.Reason(err)
}

// ---- errors ----

func errDownloadFailed(key, github, manifestFile string, cause error) error {
	return &diag.Error{
		Msg:   "Downloading library " + key + " failed: " + reasonOf(cause),
		File:  manifestFile,
		Hint:  "Check your connection and that https://github.com/" + github + " exists.",
		Cause: cause,
	}
}

func errNoSuchTag(key, github, tag, manifestFile string) error {
	return &diag.Error{
		Msg:  "Library " + key + ": " + github + " has no tag " + tag + ".",
		File: manifestFile,
		Hint: "See the tags at https://github.com/" + github + "/tags.",
	}
}

func errStatus(key string, status int, manifestFile string) error {
	return &diag.Error{
		Msg:  fmt.Sprintf("Downloading library %s failed: HTTP %d.", key, status),
		File: manifestFile,
		Hint: "Try again later.",
	}
}

func errNotATagArchive(key, manifestFile, address string, cause error) error {
	return &diag.Error{
		Msg:   "The download of library " + key + " is not a GitHub tag archive: " + reasonOf(cause),
		File:  manifestFile,
		Hint:  "Check " + address + " in a browser.",
		Cause: cause,
	}
}

func errUnsafePath(key, manifestFile, address, path string) error {
	return &diag.Error{
		Msg:  "The download of library " + key + " has an unsafe path: " + path,
		File: manifestFile,
		Hint: "Check " + address + " in a browser.",
	}
}

// The four failures of readArchive have no file and no hint: errNotATagArchive puts the library, the manifest and
// the address around what they say.

func errInvalidZip(cause error) error {
	return &diag.Error{Msg: "Invalid zip archive: " + strings.TrimPrefix(cause.Error(), "zip: ") + ".", Cause: cause}
}

func errNoCommit() error {
	return &diag.Error{Msg: "The archive's comment is not a commit SHA."}
}

func errNoSingleTop() error {
	return &diag.Error{Msg: "The archive does not have a single top folder."}
}

func errEntryUnreadable(name string, cause error) error {
	return &diag.Error{Msg: fmt.Sprintf("Invalid zip archive: %s cannot be read: %v.", name, cause), Cause: cause}
}
