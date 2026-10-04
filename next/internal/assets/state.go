package assets

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"regexp"

	"github.com/mdlsvensson/moonwell/next/internal/diag"
	"github.com/mdlsvensson/moonwell/next/internal/fsx"
	"github.com/mdlsvensson/moonwell/next/internal/manifest"
	"github.com/mdlsvensson/moonwell/next/internal/mapdir"
)

// State is the ownership file: the source-map files assets:sync owns, by in-map path, with their SHA-256.
type State struct{ Files []Owned }

// Owned is one file assets:sync owns: its in-map path as the state file spells it, and the SHA-256 of what was
// written there, in lower-case hexadecimal.
type Owned struct{ Path, Hash string }

// StateFile is the ownership file of the project at root for a map folder: .asset-state/<map folder>.json.
func StateFile(root, mapFolder string) (string, error) {
	return fsx.SafeJoin(root, ".asset-state/"+mapFolder+".json")
}

// ReadState reads the ownership file. A project without the file owns nothing: its state is empty. A file that
// is not a state is refused whole, and so is one that lists a path no asset may have: assets:sync removes the
// files a state lists, and a state that named war3map.lua would have it remove the map's script.
func ReadState(file string) (State, error) {
	data, found, err := readIfThere(file)
	if err != nil || !found {
		return State{}, err
	}
	listed, problem := listedIn(data)
	if problem != "" {
		return State{}, errInvalidState(file, problem)
	}
	var state State
	seen := map[string]bool{}
	for path, written := range listed.All() {
		owned, err := ownedFile(file, path, written)
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

// readIfThere reads a file. found is false, without an error, when there is none.
func readIfThere(file string) (data []byte, found bool, err error) {
	data, err = os.ReadFile(file)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return nil, false, nil
	case err != nil:
		return nil, false, errUnreadableState(file, err)
	}
	return data, true, nil
}

// listedIn is the files a state document lists, each path with its hash as written, in the order of the text; or
// what is wrong with the document.
func listedIn(data []byte) (files manifest.Ordered[json.RawMessage], problem string) {
	var document manifest.Ordered[json.RawMessage]
	switch {
	case !json.Valid(data):
		return files, "it is not JSON"
	case json.Unmarshal(data, &document) != nil, !isOne(document, "version"):
		// JSON that is no object has no version either.
		return files, "version must be 1"
	}
	listed, _ := document.Get("files")
	if !isObject(listed) || json.Unmarshal(listed, &files) != nil {
		return files, "files must be an object"
	}
	return files, ""
}

// isOne reports whether the member of a document under name is the number 1, however it is written. A document
// without the member, and a member that is no number, are not: null reads as 0, and nothing else reads at all.
func isOne(document manifest.Ordered[json.RawMessage], name string) bool {
	var number float64
	written, _ := document.Get(name)
	return json.Unmarshal(written, &number) == nil && number == 1
}

// isObject reports whether a JSON value is an object.
func isObject(written json.RawMessage) bool {
	return bytes.HasPrefix(bytes.TrimSpace(written), []byte("{"))
}

var sha256Hex = regexp.MustCompile(`^[a-f0-9]{64}$`)

// ownedFile is one entry of a state file as an owned file: a path an asset may have, with a SHA-256.
func ownedFile(file, path string, written json.RawMessage) (Owned, error) {
	if _, err := TargetPath(path); err != nil {
		return Owned{}, inState(err, file)
	}
	var hash string
	if json.Unmarshal(written, &hash) != nil || !sha256Hex.MatchString(hash) {
		return Owned{}, errInvalidState(file, path+" has no valid hash")
	}
	return Owned{path, hash}, nil
}

// inState turns the refusal of a path into the state file's, which listed it. Any other error stays as it is.
func inState(err error, file string) error {
	var failure *diag.Error
	if !errors.As(err, &failure) {
		return err
	}
	return errStatePath(file, failure)
}

// Bytes is the state as its file holds it: the version, then the files in the order given, with two spaces of
// indentation and a final line break. A path and a hash are written as fsx.Quoted writes them, so that a state
// file a project has committed is not written anew for the same state.
func (s State) Bytes() []byte {
	var out bytes.Buffer
	out.WriteString("{\n  \"version\": 1,\n  \"files\": {")
	for i, file := range s.Files {
		if i > 0 {
			out.WriteByte(',')
		}
		out.WriteString("\n    " + fsx.Quoted(file.Path) + ": " + fsx.Quoted(file.Hash))
	}
	if len(s.Files) > 0 {
		out.WriteString("\n  ")
	}
	out.WriteString("}\n}\n")
	return out.Bytes()
}

// ---- errors ----

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
