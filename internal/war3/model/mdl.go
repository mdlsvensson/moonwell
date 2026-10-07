package model

import (
	"strconv"
	"strings"
)

// A text model is blocks: a name, at times more words or a string, and braces around statements and more blocks.
// A statement is words and strings up to a comma or to the brace that closes its block. White space is ASCII
// white space, and a comment runs from two slashes, where a token could start, to the end of the line.
const (
	mdlSpaces   = " \t\n\v\f\r"
	mdlWordEnds = mdlSpaces + `{},"`
)

type mdlTokenKind uint8

const (
	mdlEnd mdlTokenKind = iota // after the last token
	mdlString
	mdlWord
	mdlOpen
	mdlClose
	mdlComma
)

// mdlToken is one token. text is the word, or the string without its quotes.
type mdlToken struct {
	kind mdlTokenKind
	text string
}

// mdlTokens hands out the tokens of a text model one at a time, so that a large model never has all of its tokens
// in memory.
type mdlTokens struct {
	source string
	at     int
}

// next returns the next token, of the kind mdlEnd when there is none left. file is the name its error gives.
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

// skipSpaceAndComments moves to the start of the next token, or to the end of the source.
func (t *mdlTokens) skipSpaceAndComments() {
	for t.at < len(t.source) {
		rest := t.source[t.at:]
		switch {
		case strings.IndexByte(mdlSpaces, rest[0]) >= 0:
			t.at++
		case strings.HasPrefix(rest, "//"):
			t.at += lineLength(rest)
		default:
			return
		}
	}
}

// lineLength is the number of bytes before the first line feed of text, or all of them when it has none.
func lineLength(text string) int {
	if end := strings.IndexByte(text, '\n'); end >= 0 {
		return end
	}
	return len(text)
}

// quoted reads a string, which runs to the next quote whatever stands between the two.
func (t *mdlTokens) quoted(file string) (mdlToken, error) {
	body := t.source[t.at+1:]
	end := strings.IndexByte(body, '"')
	if end < 0 {
		return mdlToken{}, errStringNeverClosed(file)
	}
	t.at += end + 2
	return mdlToken{mdlString, body[:end]}, nil
}

// word reads a word, which runs to white space, a brace, a comma or a quote.
func (t *mdlTokens) word() mdlToken {
	rest := t.source[t.at:]
	end := strings.IndexAny(rest, mdlWordEnds)
	if end < 0 {
		end = len(rest)
	}
	t.at += end
	return mdlToken{mdlWord, rest[:end]}
}

// mdlBlock is a block that is being read: its name and what its statements have said so far of the things a
// reference is made of.
type mdlBlock struct {
	name             string
	image            string // the string of the last Image statement
	path             string // the string of the last Path statement
	hasPath          bool   // a Path statement was read, with a path or with an empty string
	replaceableID    int64  // the number of the last ReplaceableId statement
	usesMDL, usesTGA bool   // the flags EmitterUsesMDL and EmitterUsesTGA
}

// set takes a statement of the block: a word alone is a flag, a word and a string name a file, and a word and a
// whole number are a number. Statements of any other shape, and of other names, say nothing about a reference.
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

// wholeNumber reads a word of digits, with or without a minus sign in front. A number too large for 64 bits is
// read as the largest or the smallest that fits.
func wholeNumber(word string) (int64, bool) {
	digits := strings.TrimPrefix(word, "-")
	if digits == "" || strings.Trim(digits, "0123456789") != "" {
		return 0, false
	}
	number, _ := strconv.ParseInt(word, 10, 64) // the error is of a number out of range, for which it returns the nearest
	return number, true
}

// adopt takes the path of a Particle block that closed inside a ParticleEmitter without a Path statement of its
// own so far: exporters write an emitter's path there.
func (b *mdlBlock) adopt(child mdlBlock) {
	if b.name == "ParticleEmitter" && !b.hasPath && child.name == "Particle" && child.hasPath {
		b.path, b.hasPath = child.path, true
	}
}

// reference returns the file a closed block references. A Bitmap always references a texture, by its image or by
// its slot; the other path-bearing blocks reference a file when they have a path.
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

// mdlReader collects the references of a text model from its tokens.
type mdlReader struct {
	file      string
	open      []mdlBlock // the blocks that are not closed yet, the outermost first
	statement []mdlToken // the words and strings since the last brace or comma
	hasHeader bool       // a Version or a Model block in no other block, which every exporter writes
	paths     []Path
}

// ReadMDL returns every file a text MDL model references, in file order. file is the name its errors give. The
// text must close every string and block it opens and have a Version or a Model block.
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

// take reads one token.
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

// openBlock opens a block named by the first word since the last brace or comma; without one, or after a string,
// the block has no name.
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

// endStatement gives the statement to the innermost open block. A statement in no block says nothing.
func (r *mdlReader) endStatement() {
	if len(r.open) > 0 {
		r.open[len(r.open)-1].set(r.statement)
	}
	r.statement = r.statement[:0]
}

// closeBlock ends the innermost open block and lists what it references.
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
		path.Path = strings.Clone(path.Path) // so that the paths do not keep the whole source in memory
		r.paths = append(r.paths, path)
	}
	return nil
}

// finish returns the references once the tokens have run out.
func (r *mdlReader) finish() ([]Path, error) {
	if len(r.open) > 0 {
		return nil, errBlockNeverClosed(r.file, r.open[len(r.open)-1].name)
	}
	if !r.hasHeader {
		return nil, errNoHeader(r.file)
	}
	return r.paths, nil
}

// ---- errors ----

func errStringNeverClosed(file string) error {
	return errNotReadable(file, "a string is never closed")
}

func errUnmatchedClose(file string) error {
	return errNotReadable(file, "a } has no matching {")
}

// errBlockNeverClosed names the innermost block that is still open at the end of the text.
func errBlockNeverClosed(file, name string) error {
	if name == "" {
		name = "unnamed"
	}
	return errNotReadable(file, "the "+name+" block is never closed")
}

func errNoHeader(file string) error {
	return errNotReadable(file, "it has no Version or Model block")
}
