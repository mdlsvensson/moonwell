package library

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/env"
	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/manifest"
)

func syncGitHub(
	ctx context.Context, e *env.Env, dirs libraryDirs, library manifest.Library, locked *lockEntry, manifestName string,
) (lockEntry, error) {
	github, tag, dir := *library.GitHub, *library.Tag, library.Dir
	if err := checkNames(dirs.key, github, tag, manifestName); err != nil {
		return lockEntry{}, err
	}
	sameTag := locked != nil && locked.GitHub == github && locked.Tag == tag && locked.Dir == dir
	if sameTag && isUpToDate(dirs, *locked) {
		return *locked, nil
	}
	commit, files, err := downloadTag(ctx, e.Fetch, dirs.key, github, tag, manifestName)
	if err != nil {
		return lockEntry{}, err
	}
	content, err := selectFiles(dirs.key, github, tag, dir, files, manifestName)
	if err != nil {
		return lockEntry{}, err
	}
	entry := newLockEntry(github, tag, dir, commit, content)
	if sameTag && hasTagMoved(*locked, entry) {
		return lockEntry{}, errMoved(dirs.key, github, tag, locked.Commit, entry.Commit)
	}
	if err := writeDownloaded(e.Root, dirs, content, entry); err != nil {
		return lockEntry{}, err
	}
	e.Log.Info("Fetched library " + dirs.key + ": " + github + " " + tag + " (" + shorten(entry.Commit, 7) + ").")
	return entry, nil
}

func checkNames(key, github, tag, manifestName string) error {
	for segment := range strings.SplitSeq(tag, "/") {
		if segment == "." || segment == ".." {
			return errNotATag(key, github, tag, manifestName)
		}
	}
	if parts := strings.Split(github, "/"); len(parts) > 1 && (parts[1] == "." || parts[1] == "..") {
		return errNotARepository(key, github, manifestName)
	}
	return nil
}

func isUpToDate(dirs libraryDirs, entry lockEntry) bool {
	data, err := os.ReadFile(filepath.Join(dirs.modules, stampFile))
	if err != nil {
		return false
	}
	stampEntry, isEntry := parseLockEntry(data)
	if !isEntry || !hasStampLayout(data) || !equalEntries(stampEntry, entry) {
		return false
	}
	return entry.Assets == nil || fsx.IsDir(dirs.assets)
}

func hasStampLayout(data []byte) bool {
	members, _ := asObject(data)
	var layout float64
	return json.Unmarshal(members["layout"], &layout) == nil && layout == stampLayout
}

func equalEntries(a, b lockEntry) bool {
	if (a.Assets == nil) != (b.Assets == nil) || (a.Assets != nil && *a.Assets != *b.Assets) {
		return false
	}
	return a.GitHub == b.GitHub && a.Tag == b.Tag && a.Dir == b.Dir && a.Commit == b.Commit && a.Files == b.Files
}

func selectFiles(key, github, tag, dir string, files []archiveFile, manifestName string) (libraryContent, error) {
	libraryFileURL := "https://github.com/" + github + "/blob/" + tag + "/" + File
	data, exists := findFile(files, File)
	libraryFile, err := parseLibraryFile(key, data, exists, libraryFileURL)
	if err != nil {
		return libraryContent{}, err
	}
	moduleDir := dir
	if moduleDir == "" && libraryFile.Dir != nil {
		moduleDir = *libraryFile.Dir
	}
	content := libraryContent{modules: filterFilesBelow(files, moduleDir, libraryFile.Assets), shipsAssets: libraryFile.Assets != nil}
	switch {
	case len(content.modules) == 0 && dir != "":
		return libraryContent{}, errNoFolderOfTheManifest(key, moduleDir, tag, manifestName)
	case len(content.modules) == 0:
		return libraryContent{}, errNoFolderOfTheLibrary(key, moduleDir, tag, libraryFileURL)
	}
	if content.shipsAssets {
		if content.assets = filterFilesBelow(files, *libraryFile.Assets, nil); len(content.assets) == 0 {
			return libraryContent{}, errNoAssetsOfTheLibrary(key, *libraryFile.Assets, tag, libraryFileURL)
		}
	}
	return content, content.checkUsable(key, manifestName)
}

func findFile(files []archiveFile, name string) (data []byte, found bool) {
	for _, file := range files {
		if file.name == name {
			return file.data, true
		}
	}
	return nil, false
}

func filterFilesBelow(files []archiveFile, dir string, except *string) []archiveFile {
	prefix := ""
	for _, segment := range strings.FieldsFunc(dir, func(r rune) bool { return r == '/' || r == '\\' }) {
		if segment != "." {
			prefix += segment + "/"
		}
	}
	var filtered []archiveFile
	for _, file := range files {
		rel, isBelow := strings.CutPrefix(file.name, prefix)
		if !isBelow || (except != nil && strings.HasPrefix(file.name, *except+"/")) || hasHiddenSegment(rel) {
			continue
		}
		filtered = append(filtered, archiveFile{rel, file.data})
	}
	return filtered
}

func newLockEntry(github, tag, dir, commit string, content libraryContent) lockEntry {
	entry := lockEntry{GitHub: github, Tag: tag, Dir: dir, Commit: commit, Files: hashFiles(content.modules)}
	if content.shipsAssets {
		hash := hashFiles(content.assets)
		entry.Assets = &hash
	}
	return entry
}

func hasTagMoved(locked, entry lockEntry) bool {
	switch {
	case entry.Commit != locked.Commit:
		return true
	case (locked.Assets == nil) != (entry.Assets == nil):
		return false
	}
	return entry.Files != locked.Files || (entry.Assets != nil && *entry.Assets != *locked.Assets)
}

func shorten(commit string, length int) string { return commit[:min(length, len(commit))] }

func writeDownloaded(root string, dirs libraryDirs, content libraryContent, entry lockEntry) error {
	err := removeStamp(dirs)
	switch {
	case err != nil:
		return err
	case content.shipsAssets:
		err = replaceDir(root, AssetsDir, dirs.key, content.assets, "")
	default:
		err = removeAssets(dirs)
	}
	if err != nil {
		return err
	}
	return replaceDir(root, ModulesDir, dirs.key, content.modules, stampText(entry))
}

func removeStamp(dirs libraryDirs) error {
	stampPath := filepath.Join(dirs.modules, stampFile)
	if info, err := fsx.Lstat(stampPath); err != nil || info == nil || info.IsDir() {
		return nil
	}
	if err := fsx.RemoveFile(stampPath); err != nil {
		return errUnremovable(modulesDirName(dirs.key)+"/"+stampFile, err)
	}
	return nil
}

func replaceDir(root, dir, key string, files []archiveFile, stamp string) error {
	displayPath := dir + "/" + key
	tempDir, err := fsx.SafeJoinNoSymlinks(root, dir+"/."+key+".tmp")
	if err != nil {
		return err
	}
	targetDir, err := fsx.SafeJoinNoSymlinks(root, displayPath)
	if err != nil {
		return err
	}
	if stamp != "" {
		files = append(slices.Clone(files), archiveFile{stampFile, []byte(stamp)})
	}
	if err := writeFiles(tempDir, files); err != nil {
		return errUnwritable(displayPath, err)
	}
	if err := fsx.RemoveAll(targetDir); err != nil {
		return errUnremovable(displayPath, err)
	}
	if err := os.Rename(tempDir, targetDir); err != nil {
		return errUnwritable(displayPath, err)
	}
	return nil
}

func writeFiles(dir string, files []archiveFile) error {
	if err := fsx.RemoveAll(dir); err != nil {
		return err
	}
	for _, file := range files {
		fullPath, err := fsx.SafeJoin(dir, file.name)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o777); err != nil {
			return err
		}
		if err := os.WriteFile(fullPath, file.data, 0o666); err != nil {
			return err
		}
	}
	return nil
}

func errNotATag(key, github, tag, manifestName string) error {
	return &diag.Error{
		Msg:  "Library " + key + ": " + tag + " is not a tag name.",
		File: manifestName,
		Hint: "Use the tag's name as it appears at https://github.com/" + github + "/tags.",
	}
}

func errNotARepository(key, github, manifestName string) error {
	return &diag.Error{
		Msg:  "Library " + key + ": " + github + " is not a GitHub repository.",
		File: manifestName,
		Hint: `Write it as "owner/repo".`,
	}
}

func errNoFolderOfTheManifest(key, dir, tag, manifestName string) error {
	return &diag.Error{
		Msg:  "Library " + key + " has no folder " + dir + " at " + tag + ".",
		File: manifestName,
		Hint: "Fix the library's dir.",
	}
}

func errNoFolderOfTheLibrary(key, dir, tag, displayPath string) error {
	return &diag.Error{
		Msg:  "Library " + key + " has no folder " + dir + " at " + tag + ".",
		File: displayPath,
		Hint: "Its " + File + " names a dir that has no files.",
	}
}

func errNoAssetsOfTheLibrary(key, assets, tag, displayPath string) error {
	return &diag.Error{
		Msg:  "Library " + key + " has no folder " + assets + " at " + tag + ".",
		File: displayPath,
		Hint: "Its " + File + " names an assets folder that has no files; report it to the library's author.",
	}
}

func errMoved(key, github, tag, from, to string) error {
	problem := "tag " + tag + " of " + github + " moved from " + shorten(from, 12) + " to " + shorten(to, 12) +
		" since " + lockFile + " recorded it."
	if from == to {
		problem = "the files of tag " + tag + " of " + github + " are not those " + lockFile + " recorded for commit " +
			shorten(to, 12) + "."
	}
	return &diag.Error{
		Msg:  "Library " + key + ": " + problem,
		File: lockFile,
		Hint: "If the move was intended, delete the library's entry from " + lockFile + " and run the command again.",
	}
}
