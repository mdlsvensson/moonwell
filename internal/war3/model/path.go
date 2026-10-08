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

func DescribePath(p Path) string {
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

func ReadPaths(data []byte, displayPath string) ([]Path, error) {
	if IsMDX(data) {
		return ReadMDX(data, displayPath)
	}
	if bytes.IndexByte(data, 0) >= 0 {
		return nil, errUnknownFormat(displayPath)
	}
	return ReadMDL(fsx.DecodeText(data), displayPath)
}

func emitterKind(usesMDL, usesTGA bool) Kind {
	if usesTGA && !usesMDL {
		return ParticleTexture
	}
	return ParticleModel
}

func errNotReadable(displayPath, problem string) error {
	return &diag.Error{
		Msg:  "Not a readable model: " + problem + ".",
		File: displayPath,
		Hint: "Re-export it from your modelling tool, or open it in a model viewer to check it.",
	}
}

func errUnknownFormat(displayPath string) error {
	return errNotReadable(displayPath, "it is neither a binary MDX nor a text MDL file")
}
