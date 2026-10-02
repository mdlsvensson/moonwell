package models

import (
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/mdlsvensson/moonwell/internal/text"
)

type mdlKind uint8

const (
	mdlString mdlKind = iota
	mdlWord
	mdlOpen
	mdlClose
	mdlComma
)

type mdlToken struct {
	kind  mdlKind
	value string
}

// mdlTokens yields the tokens of a text model one at a time, so a large model never holds all of its tokens in
// memory.
type mdlTokens struct {
	text string
	file string
	at   int
}

func (t *mdlTokens) next() (mdlToken, bool, error) {
	source := t.text
	for t.at < len(source) {
		c := source[t.at]
		r, size := rune(c), 1
		if c >= utf8.RuneSelf {
			r, size = utf8.DecodeRuneInString(source[t.at:])
		}
		switch {
		case text.IsSpace(r):
			t.at += size
		case c == '/' && t.at+1 < len(source) && source[t.at+1] == '/':
			end := strings.IndexByte(source[t.at:], '\n')
			if end < 0 {
				t.at = len(source)
			} else {
				t.at += end
			}
		case c == '{':
			t.at++
			return mdlToken{mdlOpen, "{"}, true, nil
		case c == '}':
			t.at++
			return mdlToken{mdlClose, "}"}, true, nil
		case c == ',':
			t.at++
			return mdlToken{mdlComma, ","}, true, nil
		case c == '"':
			end := strings.IndexByte(source[t.at+1:], '"')
			if end < 0 {
				return mdlToken{}, false, modelError(t.file, "a string is never closed")
			}
			value := source[t.at+1 : t.at+1+end]
			t.at += end + 2
			return mdlToken{mdlString, value}, true, nil
		default:
			end := t.at
			for end < len(source) {
				wordRune, wordSize := utf8.DecodeRuneInString(source[end:])
				if text.IsSpace(wordRune) || strings.ContainsRune(`{},"`, wordRune) {
					break
				}
				end += wordSize
			}
			value := source[t.at:end]
			t.at = end
			return mdlToken{mdlWord, value}, true, nil
		}
	}
	return mdlToken{}, false, nil
}

type mdlBlock struct {
	name    string
	strings map[string]string
	numbers map[string]int64
	flags   map[string]bool
}

// path returns the reference a closed block makes, if it is a path-bearing block with a path.
func (b *mdlBlock) path() (Path, bool) {
	path := b.strings["Path"]
	switch b.name {
	case "Bitmap":
		return Path{Kind: Texture, Path: b.strings["Image"], ReplaceableID: b.numbers["ReplaceableId"]}, true
	case "ParticleEmitter":
		kind := ParticleModel
		if b.flags["EmitterUsesTGA"] && !b.flags["EmitterUsesMDL"] {
			kind = ParticleTexture
		}
		return Path{Kind: kind, Path: path}, path != ""
	case "Attachment":
		return Path{Kind: Attachment, Path: path}, path != ""
	case "ParticleEmitterPopcorn":
		return Path{Kind: Popcorn, Path: path}, path != ""
	case "FaceFX":
		return Path{Kind: FaceEffect, Path: path}, path != ""
	}
	return Path{}, false
}

var mdlInteger = regexp.MustCompile(`^-?[0-9]+$`)

// ReadMDL returns every file a text MDL model references, in file order.
func ReadMDL(source, file string) ([]Path, error) {
	var paths []Path
	var stack []*mdlBlock
	var statement []mdlToken
	hasHeader := false // a top-level Version or Model block, which every exporter writes
	finishStatement := func() {
		if len(stack) > 0 && len(statement) > 0 && statement[0].kind == mdlWord {
			block, key := stack[len(stack)-1], statement[0].value
			switch {
			case len(statement) == 1:
				block.flags[key] = true
			case len(statement) == 2 && statement[1].kind == mdlString:
				block.strings[key] = statement[1].value
			case len(statement) == 2 && mdlInteger.MatchString(statement[1].value):
				number, _ := strconv.ParseInt(statement[1].value, 10, 64)
				block.numbers[key] = number
			}
		}
		statement = nil
	}

	tokens := &mdlTokens{text: source, file: file}
	for {
		token, ok, err := tokens.next()
		if err != nil {
			return nil, err
		}
		if !ok {
			break
		}
		switch token.kind {
		case mdlOpen:
			name := ""
			if len(statement) > 0 && statement[0].kind == mdlWord {
				name = statement[0].value
			}
			statement = nil
			if len(stack) == 0 && (name == "Version" || name == "Model") {
				hasHeader = true
			}
			stack = append(stack, &mdlBlock{
				name: name, strings: map[string]string{}, numbers: map[string]int64{}, flags: map[string]bool{},
			})
		case mdlClose:
			finishStatement()
			if len(stack) == 0 {
				return nil, modelError(file, "a } has no matching {")
			}
			block := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			// Exporters write a ParticleEmitter's Path inside its nested Particle block.
			if childPath, ok := block.strings["Path"]; ok && block.name == "Particle" && len(stack) > 0 {
				parent := stack[len(stack)-1]
				if _, has := parent.strings["Path"]; !has && parent.name == "ParticleEmitter" {
					parent.strings["Path"] = childPath
				}
			}
			if path, ok := block.path(); ok {
				paths = append(paths, path)
			}
		case mdlComma:
			finishStatement()
		default:
			statement = append(statement, token)
		}
	}
	if len(stack) > 0 {
		name := stack[len(stack)-1].name
		if name == "" {
			name = "unnamed"
		}
		return nil, modelError(file, "the "+name+" block is never closed")
	}
	if !hasHeader {
		return nil, modelError(file, "it has no Version or Model block")
	}
	return paths, nil
}
