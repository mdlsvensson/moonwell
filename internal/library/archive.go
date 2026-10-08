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

type file struct {
	name string
	data []byte
}

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

func archiveURL(github, tag string) string {
	segments := strings.Split(tag, "/")
	for i, segment := range segments {
		segments[i] = url.PathEscape(segment)
	}
	return "https://codeload.github.com/" + github + "/zip/refs/tags/" + strings.Join(segments, "/")
}

func readArchive(data []byte) (commit string, files []file, err error) {
	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
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

func commitOf(comment string) (string, error) {
	commit := fsx.TrimASCIISpace(comment)
	if len(commit) != 40 || strings.ContainsFunc(commit, notALowerHexDigit) {
		return "", errNoCommit()
	}
	return commit, nil
}

func notALowerHexDigit(r rune) bool { return !strings.ContainsRune("0123456789abcdef", r) }

func filesBelowTheTop(entries []*zip.File) ([]file, error) {
	var files []file
	place := map[string]int{}
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

func filesHash(files []file) string {
	sorted := slices.SortedFunc(slices.Values(files), func(a, b file) int { return strings.Compare(a.name, b.name) })
	var lines strings.Builder
	for _, f := range sorted {
		lines.WriteString(f.name + "\n" + fsx.SHA256Hex(f.data) + "\n")
	}
	return "sha256:" + fsx.SHA256Hex([]byte(lines.String()))
}

func reasonOf(err error) string {
	var failure *diag.Error
	if errors.As(err, &failure) {
		return failure.Msg
	}
	return fsx.Reason(err)
}

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
