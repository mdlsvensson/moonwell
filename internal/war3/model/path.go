package model

import (
	"bytes"
	"fmt"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/fsx"
)

type Kind string

const (
	Texture         Kind = "texture"
	ParticleModel   Kind = "particle model"
	ParticleTexture Kind = "particle texture"
	Attachment      Kind = "attachment"
	Popcorn         Kind = "popcorn"
	FaceEffect      Kind = "face effect"
)

type Path struct {
	Kind          Kind
	Path          string
	ReplaceableID int64
}

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

func Paths(data []byte, file string) ([]Path, error) {
	if IsMDX(data) {
		return ReadMDX(data, file)
	}
	if bytes.IndexByte(data, 0) >= 0 {
		return nil, errNeitherFormat(file)
	}
	return ReadMDL(fsx.DecodeText(data), file)
}

func emitterKind(usesMDL, usesTGA bool) Kind {
	if usesTGA && !usesMDL {
		return ParticleTexture
	}
	return ParticleModel
}

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
