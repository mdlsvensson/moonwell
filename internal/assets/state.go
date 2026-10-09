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
	stateFile := ".asset-state/" + mapFolder + ".json"
	if _, err := fsx.SafeJoinNoSymlinks(root, stateFile); err != nil {
		return "", err
	}
	return stateFile, nil
}

func ReadState(root, stateFile string) (State, error) {
	fullPath, err := fsx.SafeJoinNoSymlinks(root, stateFile)
	if err != nil {
		return State{}, err
	}
	data, found, err := readStateFile(fullPath, stateFile)
	if err != nil || !found {
		return State{}, err
	}
	files, problem := parseFileList(data)
	if problem != "" {
		return State{}, errInvalidState(stateFile, problem)
	}
	return parseOwnedFiles(stateFile, files)
}

func readStateFile(fullPath, stateFile string) (data []byte, found bool, err error) {
	if data, found, err = fsx.ReadFileIfExists(fullPath); err != nil {
		return nil, false, errUnreadableState(stateFile, err)
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
	rawFiles, _ := document.Get("files")
	if !isJSONObject(rawFiles) || json.Unmarshal(rawFiles, &files) != nil {
		return files, "files must be an object"
	}
	return files, ""
}

func isOne(document manifest.OrderedMap[json.RawMessage], name string) bool {
	var number float64
	raw, _ := document.Get(name)
	return json.Unmarshal(raw, &number) == nil && number == 1
}

func isJSONObject(raw json.RawMessage) bool {
	return bytes.HasPrefix(bytes.TrimSpace(raw), []byte("{"))
}

func parseOwnedFiles(stateFile string, files manifest.OrderedMap[json.RawMessage]) (State, error) {
	var state State
	seen := map[string]bool{}
	for path, rawHash := range files.All() {
		owned, err := parseOwned(stateFile, path, rawHash)
		if err != nil {
			return State{}, err
		}
		if seen[mapdir.Key(path)] {
			return State{}, errInvalidState(stateFile, path+" is listed twice")
		}
		seen[mapdir.Key(path)] = true
		state.Files = append(state.Files, owned)
	}
	return state, nil
}

var sha256Hex = regexp.MustCompile(`^[a-f0-9]{64}$`)

func parseOwned(stateFile, path string, rawHash json.RawMessage) (Owned, error) {
	if _, err := parseTargetPath(path); err != nil {
		return Owned{}, blameStateFile(err, stateFile)
	}
	var hash string
	if json.Unmarshal(rawHash, &hash) != nil || !sha256Hex.MatchString(hash) {
		return Owned{}, errInvalidState(stateFile, path+" has no valid hash")
	}
	return Owned{path, hash}, nil
}

func blameStateFile(err error, stateFile string) error {
	var diagErr *diag.Error
	if !errors.As(err, &diagErr) {
		return err
	}
	return errStatePath(stateFile, diagErr)
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

func errInvalidState(stateFile, problem string) error {
	return &diag.Error{Msg: "The asset ownership state is invalid: " + problem + ".", File: stateFile, Hint: stateHint}
}

func errStatePath(stateFile string, diagErr *diag.Error) error {
	return &diag.Error{
		Msg:   "The asset ownership state is invalid: " + diagErr.Msg,
		File:  stateFile,
		Hint:  stateHint,
		Cause: diagErr,
	}
}

func errUnreadableState(stateFile string, cause error) error {
	return &diag.Error{
		Msg:   "Reading the asset ownership state failed: " + fsx.Reason(cause),
		File:  stateFile,
		Cause: cause,
		Hint:  "Make sure it is a readable file, not a folder, and that no other program has it locked.",
	}
}
