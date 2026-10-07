// Package model lists the files a Warcraft III model references, from binary MDX or text MDL. It takes the
// model's bytes. It does not know which paths the game ships.
//
// Paths, ReadMDX and ReadMDL return the references in the order the model holds them, or a *diag.Error that names
// the file when the bytes are not a model they can read.
package model

import (
	"bytes"
	"fmt"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/fsx"
)

// Kind is what a model uses a referenced file for.
type Kind string

const (
	Texture         Kind = "texture"
	ParticleModel   Kind = "particle model"
	ParticleTexture Kind = "particle texture"
	Attachment      Kind = "attachment"
	Popcorn         Kind = "popcorn"
	FaceEffect      Kind = "face effect"
)

// Path is a file a model references.
type Path struct {
	Kind Kind
	// Path is empty for a replaceable texture (team colour and the like), which has only its slot.
	Path          string
	ReplaceableID int64
}

// Describe returns the path, or a label for a replaceable texture that has none.
func Describe(p Path) string {
	switch {
	case p.Path != "":
		return p.Path
	case p.ReplaceableID == 1:
		return "team colour (slot 1)"
	case p.ReplaceableID == 2:
		return "team glow (slot 2)"
	}
	return fmt.Sprintf("replaceable texture (slot %d)", p.ReplaceableID)
}

// Paths returns every file a model references: binary MDX when the bytes start with MDLX, otherwise text MDL.
// file is the name its errors give. Text is read as UTF-8, without a byte order mark at its start; bytes with a
// NUL among them are no text.
func Paths(data []byte, file string) ([]Path, error) {
	if IsMDX(data) {
		return ReadMDX(data, file)
	}
	if bytes.IndexByte(data, 0) >= 0 {
		return nil, errNeitherFormat(file)
	}
	return ReadMDL(fsx.DecodeText(data), file)
}

// emitterKind is what a particle emitter emits: a texture when it says so and does not say a model too, and a
// model otherwise.
func emitterKind(usesMDL, usesTGA bool) Kind {
	if usesTGA && !usesMDL {
		return ParticleTexture
	}
	return ParticleModel
}

// ---- errors ----

// errNotReadable says that the file cannot be read as a model, and why. The readers of both formats raise their
// errors through it.
func errNotReadable(file, problem string) error {
	return &diag.Error{
		Msg:  "Not a readable model: " + problem + ".",
		File: file,
		Hint: "Re-export it from your modelling tool, or open it in a model viewer to check it.",
	}
}

func errNeitherFormat(file string) error {
	return errNotReadable(file, "it is neither a binary MDX nor a text MDL file")
}
