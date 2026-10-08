package model

import (
	"strconv"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/fsx"
)

const mdlWordEnds = fsx.ASCIISpace + `{},"`

type mdlTokenKind uint8

const (
	mdlEnd mdlTokenKind = iota
	mdlString
	mdlWord
	mdlOpen
	mdlClose
	mdlComma
)

type mdlToken struct {
	kind mdlTokenKind
	text string
}

type mdlTokens struct {
	source string
	at     int
}

func (t *mdlTokens) next(file string) (mdlToken, error) {
	t.skipSpaceAndComments()
	if t.at == len(t.source) {
		return mdlToken{kind: mdlEnd}, nil
	}
	switch t.source[t.at] {
	case '{':
		t.at++
		return mdlToken{kind: mdlOpen}, nil
	case '}':
		t.at++
		return mdlToken{kind: mdlClose}, nil
	case ',':
		t.at++
		return mdlToken{kind: mdlComma}, nil
	case '"':
		return t.quoted(file)
	}
	return t.word(), nil
}

func (t *mdlTokens) skipSpaceAndComments() {
	for t.at < len(t.source) {
		rest := t.source[t.at:]
		switch {
		case strings.IndexByte(fsx.ASCIISpace, rest[0]) >= 0:
			t.at++
		case strings.HasPrefix(rest, "//"):
			t.at += lineLength(rest)
		default:
			return
		}
	}
}

func lineLength(text string) int {
	if end := strings.IndexByte(text, '\n'); end >= 0 {
		return end
	}
	return len(text)
}

func (t *mdlTokens) quoted(file string) (mdlToken, error) {
	body := t.source[t.at+1:]
	end := strings.IndexByte(body, '"')
	if end < 0 {
		return mdlToken{}, errStringNeverClosed(file)
	}
	t.at += end + 2
	return mdlToken{mdlString, body[:end]}, nil
}

func (t *mdlTokens) word() mdlToken {
	rest := t.source[t.at:]
	end := strings.IndexAny(rest, mdlWordEnds)
	if end < 0 {
		end = len(rest)
	}
	t.at += end
	return mdlToken{mdlWord, rest[:end]}
}

type mdlBlock struct {
	name             string
	image            string
	path             string
	hasPath          bool
	replaceableID    int64
	usesMDL, usesTGA bool
}

func (b *mdlBlock) set(statement []mdlToken) {
	if len(statement) == 0 || len(statement) > 2 || statement[0].kind != mdlWord {
		return
	}
	key := statement[0].text
	switch {
	case len(statement) == 1:
		b.usesMDL = b.usesMDL || key == "EmitterUsesMDL"
		b.usesTGA = b.usesTGA || key == "EmitterUsesTGA"
	case statement[1].kind == mdlString && key == "Image":
		b.image = statement[1].text
	case statement[1].kind == mdlString && key == "Path":
		b.path, b.hasPath = statement[1].text, true
	case statement[1].kind == mdlWord && key == "ReplaceableId":
		if number, ok := wholeNumber(statement[1].text); ok {
			b.replaceableID = number
		}
	}
}

func wholeNumber(word string) (int64, bool) {
	digits := strings.TrimPrefix(word, "-")
	if digits == "" || strings.Trim(digits, "0123456789") != "" {
		return 0, false
	}
	number, _ := strconv.ParseInt(word, 10, 64)
	return number, true
}

func (b *mdlBlock) adopt(child mdlBlock) {
	if b.name == "ParticleEmitter" && !b.hasPath && child.name == "Particle" && child.hasPath {
		b.path, b.hasPath = child.path, true
	}
}

func (b mdlBlock) reference() (Path, bool) {
	switch b.name {
	case "Bitmap":
		return Path{Kind: Texture, Path: b.image, ReplaceableID: b.replaceableID}, true
	case "ParticleEmitter":
		return Path{Kind: emitterKind(b.usesMDL, b.usesTGA), Path: b.path}, b.path != ""
	case "Attachment":
		return Path{Kind: Attachment, Path: b.path}, b.path != ""
	case "ParticleEmitterPopcorn":
		return Path{Kind: Popcorn, Path: b.path}, b.path != ""
	case "FaceFX":
		return Path{Kind: FaceEffect, Path: b.path}, b.path != ""
	}
	return Path{}, false
}

type mdlReader struct {
	file      string
	open      []mdlBlock
	statement []mdlToken
	hasHeader bool
	paths     []Path
}

func ReadMDL(source, file string) ([]Path, error) {
	reader := mdlReader{file: file}
	tokens := mdlTokens{source: source}
	for {
		token, err := tokens.next(file)
		if err != nil {
			return nil, err
		}
		if token.kind == mdlEnd {
			return reader.finish()
		}
		if err := reader.take(token); err != nil {
			return nil, err
		}
	}
}

func (r *mdlReader) take(token mdlToken) error {
	switch token.kind {
	case mdlOpen:
		r.openBlock()
	case mdlClose:
		return r.closeBlock()
	case mdlComma:
		r.endStatement()
	default:
		r.statement = append(r.statement, token)
	}
	return nil
}

func (r *mdlReader) openBlock() {
	name := ""
	if len(r.statement) > 0 && r.statement[0].kind == mdlWord {
		name = r.statement[0].text
	}
	r.statement = r.statement[:0]
	if len(r.open) == 0 && (name == "Version" || name == "Model") {
		r.hasHeader = true
	}
	r.open = append(r.open, mdlBlock{name: name})
}

func (r *mdlReader) endStatement() {
	if len(r.open) > 0 {
		r.open[len(r.open)-1].set(r.statement)
	}
	r.statement = r.statement[:0]
}

func (r *mdlReader) closeBlock() error {
	r.endStatement()
	if len(r.open) == 0 {
		return errUnmatchedClose(r.file)
	}
	closed := r.open[len(r.open)-1]
	r.open = r.open[:len(r.open)-1]
	if len(r.open) > 0 {
		r.open[len(r.open)-1].adopt(closed)
	}
	if path, ok := closed.reference(); ok {
		path.Path = strings.Clone(path.Path)
		r.paths = append(r.paths, path)
	}
	return nil
}

func (r *mdlReader) finish() ([]Path, error) {
	if len(r.open) > 0 {
		return nil, errBlockNeverClosed(r.file, r.open[len(r.open)-1].name)
	}
	if !r.hasHeader {
		return nil, errNoHeader(r.file)
	}
	return r.paths, nil
}

func errStringNeverClosed(file string) error {
	return errNotReadable(file, "a string is never closed")
}

func errUnmatchedClose(file string) error {
	return errNotReadable(file, "a } has no matching {")
}

func errBlockNeverClosed(file, name string) error {
	if name == "" {
		name = "unnamed"
	}
	return errNotReadable(file, "the "+name+" block is never closed")
}

func errNoHeader(file string) error {
	return errNotReadable(file, "it has no Version or Model block")
}
