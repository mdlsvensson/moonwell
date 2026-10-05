package build

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mdlsvensson/moonwell/next/internal/diag"
	"github.com/mdlsvensson/moonwell/next/internal/env"
	"github.com/mdlsvensson/moonwell/next/internal/fsx"
	"github.com/mdlsvensson/moonwell/next/internal/manifest"
	"github.com/mdlsvensson/moonwell/next/internal/mapdir"
	"github.com/mdlsvensson/moonwell/next/internal/war3/mpq"
	"github.com/mdlsvensson/moonwell/next/internal/war3/w3i"
)

// This file holds what follows a plan in Build and Test: the stage, the archive's place, and the archive.

const (
	// stageDir is the folder the maps are staged in, from the project folder.
	stageDir = distDir + "/stage"
	// sourcesDir is the folder of a project's gameplay, from the project folder.
	sourcesDir = "src"
	// infoName is the file of a map that says which format the map has.
	infoName = "war3map.w3i"
	// mapSuffix ends the name of a map: of its folder, and of its archive.
	mapSuffix = ".w3x"
)

// place is a file or a folder that Moonwell writes.
type place struct {
	file  string // where it is on disk
	label string // its path from the project folder, with "/": how messages name it
}

// placeOf is a place Moonwell writes, with the way to it looked at; label is its path from the project folder,
// with "/". The refusals are those of output, and that of a file where a folder on the way belongs.
func placeOf(root, label string) (place, error) {
	file, err := output(root, label)
	if err != nil {
		return place{}, err
	}
	if blocking, found := fileOnTheWay(root, label); found {
		return place{}, errFileForFolder(blocking, label)
	}
	return place{file: file, label: label}, nil
}

// fileOnTheWay is the first folder on the way to label that is a file; label is a path from the project folder,
// with "/". What stands at label itself is not looked at.
//
// output takes a file on the way for a place that nothing is at, and what a system then says of a write or a
// removal below the file differs from system to system. So the file is refused by its name, before either.
func fileOnTheWay(root, label string) (file string, found bool) {
	for at, char := range label {
		if char != '/' {
			continue
		}
		info, err := os.Stat(filepath.Join(root, filepath.FromSlash(label[:at])))
		if err == nil && !info.IsDir() {
			return label[:at], true
		}
	}
	return "", false
}

// labelOf is a file at or below the place as its path from the project folder; file is where it is on disk. A
// file that is not below the place is named as the place.
func (at place) labelOf(file string) string {
	below, err := filepath.Rel(at.file, file)
	if err != nil || below == "." || !filepath.IsLocal(below) {
		return at.label
	}
	return at.label + "/" + filepath.ToSlash(below)
}

// ---- the stage ----

// stage writes the planned map to dist/stage/<map.folder>, in place of what a build left there, and says what
// the map holds. It returns where the map is staged.
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

// stagePlace is where the project's map is staged: dist/stage/<map.folder>, a folder named as the source map
// is, since the game loads a map that is a folder by its .w3x name.
//
// A map folder below lua/ has its stage in the folder of the compile's cache, dist/stage/lua, and neither
// touches what the other wrote. A map's folder ends in .w3x, as the schema has it, and nothing of the cache is
// named so: the cache holds .hashes.json, .globals.json and one .lua for each module, below the folders of the
// module's name and, for a library's module, below .libraries and the library's key; a folder of a module's name
// holds no dot, and a key is letters, digits, "_" and "-". The compile removes the files it wrote, each by its
// name, and staging replaces the stage's own folder.
func stagePlace(p *manifest.Project) (place, error) {
	folder, err := mapFolder(p)
	if err != nil {
		return place{}, err
	}
	return placeOf(p.Root, stageDir+"/"+folder)
}

// stagingFailure is a failure of StageTo as a command reports it. A failure of the system, which is one that has
// a cause, is named from the project folder: by the stage, and by the file of it that could not be written. A
// refusal of the plan stays as it is: a stage that would replace the source map, and a planner's bug.
func stagingFailure(err error, at place) error {
	var failure *diag.Error
	if !errors.As(err, &failure) || failure.Cause == nil {
		return err
	}
	return errNotStaged(at.label, at.labelOf(failure.File), failure.Cause)
}

// sayStaged logs what the staged map holds of the project: its objects, its settings and its assets, each in a
// line where there are any, and the lines that say which of a library's files the map's own replace.
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

// ---- the archive's place ----

// clearedArchive is where the project's archive goes, with the archive of the build before removed.
func clearedArchive(p *manifest.Project) (place, error) {
	out, err := archiveOf(p)
	if err != nil {
		return place{}, err
	}
	if err := removeArchive(out); err != nil {
		return place{}, err
	}
	return out, nil
}

// archiveOf is where the project's archive goes: <build.folder>/<map.folder> from the project folder. Nothing
// need be at the place.
//
// A build removes what is at the place and writes a file there, so the place is refused, with the manifest as
// its file, where that would harm the project: a folder at the place, and a place inside the source map, where
// a link at the first folder of build.folder leads. A source map that cannot be opened is not looked at here:
// the plan refuses it, and then no archive is written.
func archiveOf(p *manifest.Project) (place, error) {
	folder, err := mapFolder(p)
	if err != nil {
		return place{}, err
	}
	into, err := buildFolder(p, folder)
	if err != nil {
		return place{}, err
	}
	out, err := placeOf(p.Root, into+"/"+folder)
	if err != nil {
		return place{}, err
	}
	if fsx.IsDir(out.file) {
		return place{}, errOutputIsAFolder(p.File, out.label)
	}
	if source, err := Source(p); err == nil && source.Holds(out.file) {
		return place{}, errOutputInSourceMap(p.File, out.label, source.Label(""))
	}
	return out, nil
}

// buildFolder is the project's build.folder as a path from the project folder, with "/"; folder is the map's,
// which names the archive in a refusal.
//
// The value is read as mapFolder reads map.folder, by the rule of schema/Project.pkl: every way the schema lets
// a folder be written names the folder, such as "dist/bin/" and "./out". What the schema refuses is refused
// here too, with the manifest as its file, for a manifest that was not checked against it: a value that starts
// at a root or has a ".." part, one that names no folder, and a folder Moonwell keeps for itself.
func buildFolder(p *manifest.Project, folder string) (string, error) {
	written := p.Build.Folder
	parts := partsOf(written)
	switch {
	case leavesItsFolder(written, parts):
		asWritten := strings.TrimRight(strings.ReplaceAll(written, `\`, "/"), "/")
		return "", errOutputOutside(p.File, asWritten+"/"+folder)
	case len(parts) == 0:
		return "", errNoBuildFolder(p.File, written)
	}
	into := strings.Join(parts, "/")
	if kept, isKept := keptFolder(parts); isKept {
		return "", errOutputInKeptFolder(p.File, into+"/"+folder, kept)
	}
	return into, nil
}

// keptFolder is the folder Moonwell reads from or stages into that a path of these parts is, or is in: maps,
// src or dist/stage, as isReservedFolder of schema/Project.pkl has them. The schema compares in lower case, which
// for these names is the lower case of ASCII: no other letter becomes one of theirs.
//
// output takes the first folder of a path as it is, a link too, so a build.folder that starts with maps would
// have a build write through a link at maps.
func keptFolder(parts []string) (kept string, found bool) {
	first := lowerASCII(parts[0])
	switch {
	case first == mapsDir, first == sourcesDir:
		return first, true
	case len(parts) > 1 && first+"/"+lowerASCII(parts[1]) == stageDir:
		return stageDir, true
	}
	return "", false
}

// lowerASCII is text with its ASCII letters in lower case, and every other byte as it is.
func lowerASCII(text string) string {
	lowered := []byte(text)
	for at, char := range lowered {
		if char >= 'A' && char <= 'Z' {
			lowered[at] = char + 'a' - 'A'
		}
	}
	return string(lowered)
}

// removeArchive removes the archive at a place. An archive that is not there is no failure.
func removeArchive(at place) error {
	err := fsx.RemoveFile(at.file)
	if err == nil {
		return nil
	}
	// fsx words a file that another program holds by its place on disk: the system's failure is its cause.
	var held *diag.Error
	if errors.As(err, &held) && held.Cause != nil {
		err = held.Cause
	}
	return errNotRemoved(at.label, err)
}

// writeArchive writes the archive to its place, with the folders on the way. An archive that could not be
// written whole is removed: a failed build leaves no archive. A failure of that removal is passed over, since
// the failure to write is the one to report.
func writeArchive(at place, archive []byte) error {
	if err := os.MkdirAll(filepath.Dir(at.file), 0o777); err != nil {
		return errArchiveNotWritten(at.label, err)
	}
	if err := os.WriteFile(at.file, archive, 0o666); err != nil {
		_ = fsx.RemoveFile(at.file)
		return errArchiveNotWritten(at.label, err)
	}
	return nil
}

// ---- the archive ----

// packInto packs the planned map and writes the archive to its place, and says so before and after: packing is
// the slow part of a build.
func packInto(e *env.Env, plan *Result, out place) error {
	e.Log.Info("Packing archive...")
	archive, err := pack(plan.Map, strings.TrimSuffix(path.Base(out.label), mapSuffix))
	if err != nil {
		return err
	}
	if err := writeArchive(out, archive); err != nil {
		return err
	}
	e.Log.Info("Built " + out.label + " (" + strconv.Itoa(len(plan.Program.Modules)) + " module(s)).")
	return nil
}

// archiveMetadata is the files an archive keeps about itself, which a map saved as a folder may hold from the
// archive it once was: they describe that archive and are never packed. The names are in lower case; an archive
// finds a file by its name in any letter case of ASCII.
var archiveMetadata = map[string]bool{"(attributes)": true, "(listfile)": true, "(signature)": true}

// pack is the planned map as an archive; name is the map's name, for the header of a map that has one.
//
// The archive is packed from the view and not from the stage, so both hold the same bytes and each file is read
// once. Its files are in the order of Folder.Files: the files the map has on disk, each folder's entries by the
// bytes of their names with a folder's own entries where the folder stands, and then the files the plan adds,
// in the order they were first planned. A map of a format that is packed without a header gets none, and any
// other gets the HM3W header before its archive.
func pack(view *mapdir.Folder, name string) ([]byte, error) {
	format, err := formatOf(view)
	if err != nil {
		return nil, err
	}
	files, err := archiveFiles(view)
	if err != nil {
		return nil, err
	}
	var options mpq.Options
	if !format.Headerless() {
		options.Prefix = mpq.HM3WHeader(name, 0, 0)
	}
	if tooLarge, fits := roomFor(int64(len(options.Prefix)), sizesOf(files)); !fits {
		return nil, errTooLarge(view, tooLarge)
	}
	archive, err := mpq.Write(files, options)
	if err != nil {
		return nil, named(err, view.Label(""), "Check the files of the source map, then try again.")
	}
	return archive, nil
}

// formatOf is the start of the map's war3map.w3i, which says how the map is packed.
func formatOf(view *mapdir.Folder) (w3i.Header, error) {
	info, found, err := view.Read(infoName)
	switch {
	case err != nil:
		return w3i.Header{}, err
	case !found:
		return w3i.Header{}, errNoMapInfo(view.Label(""))
	}
	format, err := w3i.ReadHeader(info)
	if err != nil {
		return w3i.Header{}, named(err, view.Label(infoName), "Save the map again in World Editor.")
	}
	return format, nil
}

// archiveFiles is the files of the view as an archive names them, with "\" for "/", in the view's order and
// without the files an archive keeps about itself.
func archiveFiles(view *mapdir.Folder) ([]mpq.File, error) {
	var files []mpq.File
	for _, name := range view.Files() {
		if archiveMetadata[lowerASCII(name)] {
			continue
		}
		data, found, err := view.Read(name)
		if err != nil {
			return nil, err
		}
		if !found {
			// A plain error: Files lists the files the view has, and Read finds each of them, so a file that is
			// listed and not found is a mistake in Moonwell and nothing the user can put right.
			return nil, fmt.Errorf("build.pack: the planned map lists %s and has no such file", name)
		}
		files = append(files, mpq.File{Name: strings.ReplaceAll(name, "/", `\`), Data: data})
	}
	return files, nil
}

// named is a failure of a package that knows no file, with the file it is about and how to put it right: an
// expected failure gets the file, and the hint where it has none. Any other error stays as it is.
func named(err error, file, hint string) error {
	var failure *diag.Error
	if !errors.As(err, &failure) {
		return err
	}
	withFile := *failure
	withFile.File = file
	if withFile.Hint == "" {
		withFile.Hint = hint
	}
	return &withFile
}

// ---- the size an archive can have ----

// An archive as war3/mpq lays one out: a header, the data of each file and of the list of the files, the hash
// table and the block table. A file's data is its bytes in sectors, behind a word for where each sector starts
// and one for where the last ends.
const (
	archiveHeader = 32   // the header, in bytes
	sectorBytes   = 4096 // a sector of a file, before it is compressed
	wordBytes     = 4    // a position or a size
	entryBytes    = 16   // a slot of the hash table, and a block of the block table
	fewestSlots   = 16   // the smallest hash table
	// largestWord is the largest number a word holds: the format writes every position and size as one.
	largestWord = 1<<32 - 1
)

// sized is a file of an archive by its name there and the number of its bytes.
type sized struct {
	name string
	size int64
}

// sizesOf is the files by their sizes, in their order.
func sizesOf(files []mpq.File) []sized {
	sizes := make([]sized, len(files))
	for at, file := range files {
		sizes[at] = sized{name: file.Name, size: int64(len(file.Data))}
	}
	return sizes
}

// roomFor reports whether the format can hold an archive of the files behind a prefix of so many bytes. Where it
// cannot, tooLarge is the first file that is too large by itself, and "" where the files are too large together.
//
// The format writes as a word: the size of each file; where each file's data starts and how long it is, counted
// from the archive's header; where the two tables start; and the archive's size, from its header to the end of
// the block table, which is the largest of them. So a file must not be larger than a word holds, and the archive
// must not be: it is counted with its prefix, the header of 512 bytes that a map of an older format has, since
// a reader finds a position by adding where the archive starts, and holds the sum in a word too. The archive
// is counted at its largest, with no sector compressed, so the answer depends on the sizes alone.
func roomFor(prefix int64, files []sized) (tooLarge string, fits bool) {
	for _, file := range files {
		if file.size > largestWord {
			return file.name, false
		}
	}
	return "", largestArchive(prefix, files) <= largestWord
}

// largestArchive is the most bytes an archive of the files takes, with its prefix: the bytes it takes when no
// sector of it is stored shorter than it is.
func largestArchive(prefix int64, files []sized) int64 {
	total := prefix + archiveHeader
	var list int64
	for _, file := range files {
		total += largestStored(file.size)
		list += int64(len(file.name)) + 2 // the file's line of the list: its name and "\r\n"
	}
	entries := int64(len(files)) + 1 // the files, and the list of them
	return total + largestStored(list) + (hashSlots(entries)+entries)*entryBytes
}

// largestStored is the most bytes the data of a file of size bytes takes in an archive: its bytes, and a word
// for each sector and one more. A file without a byte takes none.
func largestStored(size int64) int64 {
	if size == 0 {
		return 0
	}
	sectors := (size + sectorBytes - 1) / sectorBytes
	return size + (sectors+1)*wordBytes
}

// hashSlots is the size of the hash table of an archive with so many entries: the smallest power of two, from
// sixteen, that leaves a third of its slots free.
func hashSlots(entries int64) int64 {
	slots := int64(fewestSlots)
	for slots*2 < entries*3 {
		slots *= 2
	}
	return slots
}

// ---- errors ----

func errFileForFolder(file, wanted string) error {
	return &diag.Error{
		Msg:  file + " is a file, not a folder.",
		File: file,
		Hint: "Moonwell writes " + wanted + " below it: remove or rename the file, then try again.",
	}
}

// stagingHint ends a failure to write the stage.
const stagingHint = "Close Warcraft III or World Editor if they have dist/stage open, then retry."

func errNotStaged(stage, file string, cause error) error {
	return &diag.Error{
		Msg:   "Staging the map into " + stage + " failed: " + fsx.Reason(cause),
		File:  file,
		Hint:  stagingHint,
		Cause: cause,
	}
}

const (
	// insideHint ends a refusal of a build.folder that names no folder of the project.
	insideHint = "Set build.folder to a folder inside the project, such as dist/bin."
	// outputOnlyHint ends a refusal of a build.folder whose folder holds more than what a build writes.
	outputOnlyHint = "Set build.folder to a folder that only holds build output, such as dist/bin."
)

func errOutputOutside(manifestFile, output string) error {
	return &diag.Error{
		Msg:  "The build output " + output + " is outside the project.",
		File: manifestFile,
		Hint: insideHint,
	}
}

func errNoBuildFolder(manifestFile, written string) error {
	return &diag.Error{Msg: `build.folder names no folder: "` + written + `".`, File: manifestFile, Hint: insideHint}
}

func errOutputInKeptFolder(manifestFile, output, kept string) error {
	return &diag.Error{
		Msg:  "The build output " + output + " is in " + kept + "/, a folder Moonwell reads from or stages into.",
		File: manifestFile,
		Hint: outputOnlyHint,
	}
}

func errOutputIsAFolder(manifestFile, output string) error {
	return &diag.Error{
		Msg:  "The build output " + output + " is a directory; refusing to replace it.",
		File: manifestFile,
		Hint: outputOnlyHint,
	}
}

func errOutputInSourceMap(manifestFile, output, sourceMap string) error {
	return &diag.Error{
		Msg:  "The build output " + output + " is inside the source map " + sourceMap + ", where a link leads.",
		File: manifestFile,
		Hint: "Replace the link in build.folder with a real folder, or set build.folder to a folder that only holds " +
			"build output, such as dist/bin.",
	}
}

// archiveHint ends a failure to remove or to write the archive.
const archiveHint = "Close Warcraft III or World Editor if they have the archive open, and make sure that its " +
	"folder can be written, then retry."

func errNotRemoved(archive string, cause error) error {
	return &diag.Error{
		Msg:   "Removing " + archive + " failed: " + fsx.Reason(cause),
		File:  archive,
		Hint:  archiveHint,
		Cause: cause,
	}
}

func errArchiveNotWritten(archive string, cause error) error {
	return &diag.Error{
		Msg:   "Writing " + archive + " failed: " + fsx.Reason(cause),
		File:  archive,
		Hint:  archiveHint,
		Cause: cause,
	}
}

func errNoMapInfo(mapLabel string) error {
	return &diag.Error{
		Msg:  infoName + " is missing from the map folder.",
		File: mapLabel,
		Hint: "Save the source map from World Editor in folder format.",
	}
}

// errTooLarge is the refusal of a map the format cannot hold: file is the file of the archive that is too large
// by itself, by its name there, and "" where the map is too large as a whole.
func errTooLarge(view *mapdir.Folder, file string) error {
	limit := strconv.FormatInt(largestWord, 10) + " bytes"
	if file == "" {
		return &diag.Error{
			Msg:  "The map is too large to pack: an archive holds at most " + limit + ".",
			File: view.Label(""),
			Hint: "Take files out of the map or out of assets/.",
		}
	}
	inMap := view.Name(strings.ReplaceAll(file, `\`, "/"))
	return &diag.Error{
		Msg:  inMap + " is too large to pack: a file of an archive holds at most " + limit + ".",
		File: view.Label(inMap),
		Hint: "Take the file out of the map or out of assets/, or make it smaller.",
	}
}
