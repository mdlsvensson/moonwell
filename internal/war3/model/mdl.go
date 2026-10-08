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

type mdlLexer struct {
	source string
	pos    int
}

func (t *mdlLexer) next(displayPath string) (mdlToken, error) {
	t.skipSpaceAndComments()
	if t.pos == len(t.source) {
		return mdlToken{kind: mdlEnd}, nil
	}
	switch t.source[t.pos] {
	case '{':
		t.pos++
		return mdlToken{kind: mdlOpen}, nil
	case '}':
		t.pos++
		return mdlToken{kind: mdlClose}, nil
	case ',':
		t.pos++
		return mdlToken{kind: mdlComma}, nil
	case '"':
		return t.readString(displayPath)
	}
	return t.readWord(), nil
}

func (t *mdlLexer) skipSpaceAndComments() {
	for t.pos < len(t.source) {
		rest := t.source[t.pos:]
		switch {
		case strings.IndexByte(fsx.ASCIISpace, rest[0]) >= 0:
			t.pos++
		case strings.HasPrefix(rest, "//"):
			t.pos += lineLength(rest)
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

func (t *mdlLexer) readString(displayPath string) (mdlToken, error) {
	body := t.source[t.pos+1:]
	end := strings.IndexByte(body, '"')
	if end < 0 {
		return mdlToken{}, errStringNeverClosed(displayPath)
	}
	t.pos += end + 2
	return mdlToken{mdlString, body[:end]}, nil
}

func (t *mdlLexer) readWord() mdlToken {
	rest := t.source[t.pos:]
	end := strings.IndexAny(rest, mdlWordEnds)
	if end < 0 {
		end = len(rest)
	}
	t.pos += end
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

func (b *mdlBlock) applyStatement(statement []mdlToken) {
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
		if number, ok := parseInt(statement[1].text); ok {
			b.replaceableID = number
		}
	}
}

func parseInt(word string) (int64, bool) {
	digits := strings.TrimPrefix(word, "-")
	if digits == "" || strings.Trim(digits, "0123456789") != "" {
		return 0, false
	}
	number, _ := strconv.ParseInt(word, 10, 64)
	return number, true
}

func (b *mdlBlock) mergeChild(child mdlBlock) {
	if b.name == "ParticleEmitter" && !b.hasPath && child.name == "Particle" && child.hasPath {
		b.path, b.hasPath = child.path, true
	}
}

func (b mdlBlock) toPath() (Path, bool) {
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
	displayPath string
	openBlocks  []mdlBlock
	statement   []mdlToken
	hasHeader   bool
	paths       []Path
}

func ReadMDL(source, displayPath string) ([]Path, error) {
	reader := mdlReader{displayPath: displayPath}
	tokens := mdlLexer{source: source}
	for {
		token, err := tokens.next(displayPath)
		if err != nil {
			return nil, err
		}
		if token.kind == mdlEnd {
			return reader.finish()
		}
		if err := reader.handleToken(token); err != nil {
			return nil, err
		}
	}
}

func (r *mdlReader) handleToken(token mdlToken) error {
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
	if len(r.openBlocks) == 0 && (name == "Version" || name == "Model") {
		r.hasHeader = true
	}
	r.openBlocks = append(r.openBlocks, mdlBlock{name: name})
}

func (r *mdlReader) endStatement() {
	if len(r.openBlocks) > 0 {
		r.openBlocks[len(r.openBlocks)-1].applyStatement(r.statement)
	}
	r.statement = r.statement[:0]
}

func (r *mdlReader) closeBlock() error {
	r.endStatement()
	if len(r.openBlocks) == 0 {
		return errUnmatchedClose(r.displayPath)
	}
	closed := r.openBlocks[len(r.openBlocks)-1]
	r.openBlocks = r.openBlocks[:len(r.openBlocks)-1]
	if len(r.openBlocks) > 0 {
		r.openBlocks[len(r.openBlocks)-1].mergeChild(closed)
	}
	if path, ok := closed.toPath(); ok {
		path.Path = strings.Clone(path.Path)
		r.paths = append(r.paths, path)
	}
	return nil
}

func (r *mdlReader) finish() ([]Path, error) {
	if len(r.openBlocks) > 0 {
		return nil, errBlockNeverClosed(r.displayPath, r.openBlocks[len(r.openBlocks)-1].name)
	}
	if !r.hasHeader {
		return nil, errNoHeader(r.displayPath)
	}
	return r.paths, nil
}

func errStringNeverClosed(displayPath string) error {
	return errNotReadable(displayPath, "a string is never closed")
}

func errUnmatchedClose(displayPath string) error {
	return errNotReadable(displayPath, "a } has no matching {")
}

func errBlockNeverClosed(displayPath, name string) error {
	if name == "" {
		name = "unnamed"
	}
	return errNotReadable(displayPath, "the "+name+" block is never closed")
}

func errNoHeader(displayPath string) error {
	return errNotReadable(displayPath, "it has no Version or Model block")
}
