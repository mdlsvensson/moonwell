package library

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"slices"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/text"
)

// Files is a set of files by POSIX path that keeps the order they were added in.
type Files struct {
	names []string
	data  map[string][]byte
}

// NewFiles returns an empty set.
func NewFiles() *Files { return &Files{data: map[string][]byte{}} }

// Set adds or replaces a file; a replaced file keeps its place.
func (f *Files) Set(name string, data []byte) {
	if _, ok := f.data[name]; !ok {
		f.names = append(f.names, name)
	}
	f.data[name] = data
}

// Get returns a file's bytes.
func (f *Files) Get(name string) ([]byte, bool) {
	data, ok := f.data[name]
	return data, ok
}

// Has reports whether the set has a file of this name.
func (f *Files) Has(name string) bool {
	_, ok := f.data[name]
	return ok
}

// Names returns the paths in the order they were added.
func (f *Files) Names() []string { return f.names }

// Len is the number of files.
func (f *Files) Len() int { return len(f.names) }

var commitSHA = regexp.MustCompile(`^[0-9a-f]{40}$`)

// ReadGitHubArchive reads a GitHub tag archive: its files, without their single top folder (whose name GitHub
// derives from the repository and tag), and the commit SHA from the zip comment. Folder entries are skipped, so
// every name is a file.
func ReadGitHubArchive(data []byte) (commit string, files *Files, err error) {
	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	// An entry name that leaves the archive's folder is not the reader's failure: DownloadTag refuses it by name.
	if err != nil && !errors.Is(err, zip.ErrInsecurePath) {
		return "", nil, &diag.Error{Msg: "Invalid zip archive: " + strings.TrimPrefix(err.Error(), "zip: ") + "."}
	}
	commit = text.Trim(archive.Comment)
	if !commitSHA.MatchString(commit) {
		return "", nil, &diag.Error{Msg: "The archive's comment is not a commit SHA."}
	}
	files = NewFiles()
	top, hasTop := "", false
	for _, entry := range archive.File {
		name := entry.Name
		if strings.HasSuffix(name, "/") {
			continue
		}
		first, rest, found := strings.Cut(name, "/")
		if !found || (hasTop && first != top) {
			return "", nil, &diag.Error{Msg: "The archive does not have a single top folder."}
		}
		top, hasTop = first, true
		reader, err := entry.Open()
		if err != nil {
			return "", nil, &diag.Error{Msg: fmt.Sprintf("Invalid zip archive: %s cannot be read: %v.", name, err)}
		}
		content, err := io.ReadAll(reader)
		reader.Close()
		if err != nil {
			return "", nil, &diag.Error{Msg: fmt.Sprintf("Invalid zip archive: %s cannot be read: %v.", name, err)}
		}
		files.Set(rest, content)
	}
	return commit, files, nil
}

// FilesHash is "sha256:" and the SHA-256 of `<path>\n<sha256 of its bytes>\n` for each file, sorted by path.
func FilesHash(files *Files) string {
	names := slices.Clone(files.Names())
	text.Sort(names)
	var listing strings.Builder
	for _, name := range names {
		data, _ := files.Get(name)
		listing.WriteString(name + "\n" + fsx.SHA256Hex(data) + "\n")
	}
	return "sha256:" + fsx.SHA256Hex([]byte(listing.String()))
}

// Fetch downloads a URL: the response's status and body. Tests substitute it.
type Fetch func(ctx context.Context, url string) (status int, body []byte, err error)

// HTTPFetch returns the Fetch that downloads with client.
func HTTPFetch(client *http.Client) Fetch {
	return func(ctx context.Context, url string) (int, []byte, error) {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return 0, nil, err
		}
		response, err := client.Do(request)
		if err != nil {
			return 0, nil, err
		}
		defer response.Body.Close()
		body, err := io.ReadAll(response.Body)
		if err != nil {
			return 0, nil, err
		}
		return response.StatusCode, body, nil
	}
}

// encodeURIComponent is JavaScript's function of that name.
func encodeURIComponent(s string) string {
	const unreserved = "-_.!~*'()"
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || strings.IndexByte(unreserved, c) >= 0 {
			b.WriteByte(c)
		} else {
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}

// ArchiveURL is the address of the zip archive of a GitHub tag; a tag's "/" stays a path separator.
func ArchiveURL(github, tag string) string {
	return "https://codeload.github.com/" + github + "/zip/refs/tags/" +
		strings.ReplaceAll(encodeURIComponent(tag), "%2F", "/")
}

// reasonOf is the text of a failure inside another message.
func reasonOf(err error) string {
	var userError *diag.Error
	if errors.As(err, &userError) {
		return userError.Msg
	}
	return fsx.Reason(err)
}

// isSafePath reports whether path is a relative POSIX path of plain names: no leading "/", no empty, "." or ".."
// segment, no "\" or ":".
func isSafePath(path string) bool {
	if strings.HasPrefix(path, "/") || strings.ContainsAny(path, `\:`) {
		return false
	}
	for _, segment := range strings.Split(path, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return false
		}
	}
	return true
}

// DownloadTag downloads a tag of library key from GitHub: its commit and files, without the archive's top folder,
// every path checked to stay inside the library's folder. Failures name manifest.
func DownloadTag(ctx context.Context, key, github, tag, manifest string, fetch Fetch) (commit string, files *Files, err error) {
	url := ArchiveURL(github, tag)
	status, body, err := fetch(ctx, url)
	if err != nil {
		return "", nil, &diag.Error{
			Msg:   "Downloading library " + key + " failed: " + reasonOf(err),
			File:  manifest,
			Cause: err,
			Hint:  "Check your connection and that https://github.com/" + github + " exists.",
		}
	}
	if status == 404 {
		return "", nil, &diag.Error{
			Msg:  "Library " + key + ": " + github + " has no tag " + tag + ".",
			File: manifest,
			Hint: "See the tags at https://github.com/" + github + "/tags.",
		}
	}
	if status < 200 || status > 299 {
		return "", nil, &diag.Error{
			Msg: fmt.Sprintf("Downloading library %s failed: HTTP %d.", key, status), File: manifest, Hint: "Try again later.",
		}
	}
	hint := "Check " + url + " in a browser."
	commit, files, err = ReadGitHubArchive(body)
	if err != nil {
		return "", nil, &diag.Error{
			Msg:   "The download of library " + key + " is not a GitHub tag archive: " + reasonOf(err),
			File:  manifest,
			Cause: err,
			Hint:  hint,
		}
	}
	for _, path := range files.Names() {
		if !isSafePath(path) {
			return "", nil, &diag.Error{
				Msg: "The download of library " + key + " has an unsafe path: " + path, File: manifest, Hint: hint,
			}
		}
	}
	return commit, files, nil
}
