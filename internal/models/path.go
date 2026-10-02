// Package models reads the files a Warcraft III model references, from binary MDX and text MDL, and knows which
// paths the game ships.
package models

import (
	"bytes"
	"fmt"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/text"
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

func modelError(file, problem string) error {
	return &diag.Error{
		Msg:  "Not a readable model: " + problem + ".",
		File: file,
		Hint: "Re-export it from your modelling tool, or open it in a model viewer to check it.",
	}
}

// Paths returns every file a model references: binary MDX when the bytes start with MDLX, otherwise text MDL.
func Paths(data []byte, file string) ([]Path, error) {
	if IsMDX(data) {
		return ReadMDX(data, file)
	}
	if bytes.IndexByte(data, 0) >= 0 {
		return nil, modelError(file, "it is neither a binary MDX nor a text MDL file")
	}
	return ReadMDL(text.Decode(data), file)
}
