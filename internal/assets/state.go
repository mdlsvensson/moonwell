package assets

import (
	"bytes"
	"encoding/json"
	"errors"
	"regexp"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/manifest"
	"github.com/mdlsvensson/moonwell/internal/mapdir"
)

type State struct{ Files []Owned }

type Owned struct{ Path, Hash string }

func StateFilePath(root, mapFolder string) (string, error) {
	file := ".asset-state/" + mapFolder + ".json"
	if _, err := fsx.SafeJoinNoSymlinks(root, file); err != nil {
		return "", err
	}
	return file, nil
}

func ReadState(root, file string) (State, error) {
	place, err := fsx.SafeJoinNoSymlinks(root, file)
	if err != nil {
		return State{}, err
	}
	data, found, err := readStateFile(place, file)
	if err != nil || !found {
		return State{}, err
	}
	listed, problem := parseFileList(data)
	if problem != "" {
		return State{}, errInvalidState(file, problem)
	}
	var state State
	seen := map[string]bool{}
	for path, written := range listed.All() {
		owned, err := parseOwned(file, path, written)
		if err != nil {
			return State{}, err
		}
		if seen[mapdir.Key(path)] {
			return State{}, errInvalidState(file, path+" is listed twice")
		}
		seen[mapdir.Key(path)] = true
		state.Files = append(state.Files, owned)
	}
	return state, nil
}

func readStateFile(place, file string) (data []byte, found bool, err error) {
	if data, found, err = fsx.ReadFileIfExists(place); err != nil {
		return nil, false, errUnreadableState(file, err)
	}
	return data, found, nil
}

func parseFileList(data []byte) (files manifest.OrderedMap[json.RawMessage], problem string) {
	var document manifest.OrderedMap[json.RawMessage]
	switch {
	case !json.Valid(data):
		return files, "it is not JSON"
	case json.Unmarshal(data, &document) != nil, !isOne(document, "version"):
		return files, "version must be 1"
	}
	listed, _ := document.Get("files")
	if !isJSONObject(listed) || json.Unmarshal(listed, &files) != nil {
		return files, "files must be an object"
	}
	return files, ""
}

func isOne(document manifest.OrderedMap[json.RawMessage], name string) bool {
	var number float64
	written, _ := document.Get(name)
	return json.Unmarshal(written, &number) == nil && number == 1
}

func isJSONObject(written json.RawMessage) bool {
	return bytes.HasPrefix(bytes.TrimSpace(written), []byte("{"))
}

var sha256Hex = regexp.MustCompile(`^[a-f0-9]{64}$`)

func parseOwned(file, path string, written json.RawMessage) (Owned, error) {
	if _, err := parseTargetPath(path); err != nil {
		return Owned{}, blameStateFile(err, file)
	}
	var hash string
	if json.Unmarshal(written, &hash) != nil || !sha256Hex.MatchString(hash) {
		return Owned{}, errInvalidState(file, path+" has no valid hash")
	}
	return Owned{path, hash}, nil
}

func blameStateFile(err error, file string) error {
	var failure *diag.Error
	if !errors.As(err, &failure) {
		return err
	}
	return errStatePath(file, failure)
}

func (s State) Encode() []byte {
	var out bytes.Buffer
	out.WriteString("{\n  \"version\": 1,\n  \"files\": {")
	for i, file := range s.Files {
		if i > 0 {
			out.WriteByte(',')
		}
		out.WriteString("\n    " + fsx.QuoteJSON(file.Path) + ": " + fsx.QuoteJSON(file.Hash))
	}
	if len(s.Files) > 0 {
		out.WriteString("\n  ")
	}
	out.WriteString("}\n}\n")
	return out.Bytes()
}

const stateHint = "Restore it from version control. It records which map files assets:sync owns."

func errInvalidState(file, problem string) error {
	return &diag.Error{Msg: "The asset ownership state is invalid: " + problem + ".", File: file, Hint: stateHint}
}

func errStatePath(file string, failure *diag.Error) error {
	return &diag.Error{
		Msg:   "The asset ownership state is invalid: " + failure.Msg,
		File:  file,
		Hint:  stateHint,
		Cause: failure,
	}
}

func errUnreadableState(file string, cause error) error {
	return &diag.Error{
		Msg:   "Reading the asset ownership state failed: " + fsx.Reason(cause),
		File:  file,
		Cause: cause,
		Hint:  "Make sure it is a readable file, not a folder, and that no other program has it locked.",
	}
}
