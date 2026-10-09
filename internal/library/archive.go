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

type archiveFile struct {
	name string
	data []byte
}

func downloadTag(
	ctx context.Context, fetch env.FetchFunc, key, github, tag, manifestName string,
) (commit string, files []archiveFile, err error) {
	address := archiveURL(github, tag)
	body, err := fetchTag(ctx, fetch, address, key, github, tag, manifestName)
	if err != nil {
		return "", nil, err
	}
	commit, files, err = readArchive(body)
	if err != nil {
		return "", nil, errNotATagArchive(key, manifestName, address, err)
	}
	for _, file := range files {
		if !isInsideLibrary(file.name) {
			return "", nil, errUnsafePath(key, manifestName, address, file.name)
		}
	}
	return commit, files, nil
}

func fetchTag(ctx context.Context, fetch env.FetchFunc, address, key, github, tag, manifestName string) ([]byte, error) {
	status, body, err := fetch(ctx, address)
	switch {
	case err != nil:
		return nil, errDownloadFailed(key, github, manifestName, err)
	case status == 404:
		return nil, errNoSuchTag(key, github, tag, manifestName)
	case status < 200 || status > 299:
		return nil, errStatus(key, status, manifestName)
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

func readArchive(data []byte) (commit string, files []archiveFile, err error) {
	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil && !errors.Is(err, zip.ErrInsecurePath) {
		return "", nil, errInvalidZip(err)
	}
	if commit, err = parseCommit(archive.Comment); err != nil {
		return "", nil, err
	}
	if files, err = filesBelowRoot(archive.File); err != nil {
		return "", nil, err
	}
	return commit, files, nil
}

func parseCommit(comment string) (string, error) {
	commit := fsx.TrimASCIISpace(comment)
	if len(commit) != 40 || strings.ContainsFunc(commit, isNotLowerHex) {
		return "", errNoCommit()
	}
	return commit, nil
}

func isNotLowerHex(r rune) bool { return !strings.ContainsRune("0123456789abcdef", r) }

func filesBelowRoot(entries []*zip.File) ([]archiveFile, error) {
	var files []archiveFile
	indexByPath := map[string]int{}
	rootDir, hasRoot := "", false
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name, "/") {
			continue
		}
		firstSegment, path, found := strings.Cut(entry.Name, "/")
		if !found || (hasRoot && firstSegment != rootDir) {
			return nil, errNoSingleRoot()
		}
		rootDir, hasRoot = firstSegment, true
		data, err := readZipFile(entry)
		if err != nil {
			return nil, err
		}
		if index, ok := indexByPath[path]; ok {
			files[index].data = data
			continue
		}
		indexByPath[path] = len(files)
		files = append(files, archiveFile{path, data})
	}
	return files, nil
}

func readZipFile(entry *zip.File) ([]byte, error) {
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

func hashFiles(files []archiveFile) string {
	sorted := slices.SortedFunc(slices.Values(files), func(a, b archiveFile) int { return strings.Compare(a.name, b.name) })
	var lines strings.Builder
	for _, file := range sorted {
		lines.WriteString(file.name + "\n" + fsx.SHA256Hex(file.data) + "\n")
	}
	return "sha256:" + fsx.SHA256Hex([]byte(lines.String()))
}

func describeError(err error) string {
	var diagErr *diag.Error
	if errors.As(err, &diagErr) {
		return diagErr.Msg
	}
	return fsx.Reason(err)
}

func errDownloadFailed(key, github, manifestName string, cause error) error {
	return &diag.Error{
		Msg:   "Downloading library " + key + " failed: " + describeError(cause),
		File:  manifestName,
		Hint:  "Check your connection and that https://github.com/" + github + " exists.",
		Cause: cause,
	}
}

func errNoSuchTag(key, github, tag, manifestName string) error {
	return &diag.Error{
		Msg:  "Library " + key + ": " + github + " has no tag " + tag + ".",
		File: manifestName,
		Hint: "See the tags at https://github.com/" + github + "/tags.",
	}
}

func errStatus(key string, status int, manifestName string) error {
	return &diag.Error{
		Msg:  fmt.Sprintf("Downloading library %s failed: HTTP %d.", key, status),
		File: manifestName,
		Hint: "Try again later.",
	}
}

func errNotATagArchive(key, manifestName, address string, cause error) error {
	return &diag.Error{
		Msg:   "The download of library " + key + " is not a GitHub tag archive: " + describeError(cause),
		File:  manifestName,
		Hint:  "Check " + address + " in a browser.",
		Cause: cause,
	}
}

func errUnsafePath(key, manifestName, address, path string) error {
	return &diag.Error{
		Msg:  "The download of library " + key + " has an unsafe path: " + path,
		File: manifestName,
		Hint: "Check " + address + " in a browser.",
	}
}

func errInvalidZip(cause error) error {
	return &diag.Error{Msg: "Invalid zip archive: " + strings.TrimPrefix(cause.Error(), "zip: ") + ".", Cause: cause}
}

func errNoCommit() error {
	return &diag.Error{Msg: "The archive's comment is not a commit SHA."}
}

func errNoSingleRoot() error {
	return &diag.Error{Msg: "The archive does not have a single top folder."}
}

func errEntryUnreadable(name string, cause error) error {
	return &diag.Error{Msg: fmt.Sprintf("Invalid zip archive: %s cannot be read: %v.", name, cause), Cause: cause}
}
