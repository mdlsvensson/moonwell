package objects

import (
	"bytes"
	"encoding/json"
	"strings"

	"github.com/mdlsvensson/moonwell/next/internal/manifest"
	"github.com/mdlsvensson/moonwell/next/internal/war3/objmod"
)

// EvalJSON is what objects:eval prints: every category in order, each object by its key with its id, base,
// source and fields. The objects of a category come in the order given, and the text ends without a line break.
func EvalJSON(resolved []Resolved) []byte {
	var p printer
	p.open('{')
	for _, category := range manifest.Categories {
		p.key(string(category))
		p.open('{')
		for _, object := range resolved {
			if object.Category == category {
				p.key(object.Key)
				p.object(object)
			}
		}
		p.close('}')
	}
	p.close('}')
	return p.out.Bytes()
}

// object prints an object with its fields in their order.
func (p *printer) object(object Resolved) {
	p.open('{')
	p.text("id", object.ID)
	p.text("base", object.Base)
	p.text("source", object.Source)
	p.key("fields")
	p.open('[')
	for _, field := range object.Fields {
		p.member()
		p.field(field)
	}
	p.close(']')
	p.close('}')
}

// typeNames is how a value's type is printed: as the metadata names the way a file stores a value.
var typeNames = map[objmod.ValueType]string{
	objmod.Int: "int", objmod.Real: "real", objmod.Unreal: "unreal", objmod.String: "string",
}

// field prints a field: where it is written, how its value is stored, and the value.
func (p *printer) field(field Field) {
	p.open('{')
	p.text("rawcode", field.ID)
	p.text("name", field.Name)
	p.number("level", float64(field.Level))
	p.number("column", float64(field.Column))
	p.truth("skin", field.Skin)
	p.text("type", typeNames[field.Value.Type])
	if field.Value.Type == objmod.String {
		p.text("value", field.Value.Text)
	} else {
		p.number("value", field.Value.Number)
	}
	p.close('}')
}

// ---- the JSON text ----

// printer writes JSON text with two spaces of indentation: every member of an object or a list on a line of its
// own, and an object or a list without members as {} or [].
type printer struct {
	out   bytes.Buffer
	depth int  // how many objects and lists are open
	bare  bool // the innermost open object or list has no member yet
}

// open starts an object or a list with its bracket.
func (p *printer) open(bracket byte) {
	p.out.WriteByte(bracket)
	p.depth++
	p.bare = true
}

// close ends the innermost object or list with its bracket, on a line of its own when it has members. What it
// ends is a member of the one around it, which so has one.
func (p *printer) close(bracket byte) {
	p.depth--
	if !p.bare {
		p.line()
	}
	p.out.WriteByte(bracket)
	p.bare = false
}

// line starts a line at the depth of what is open.
func (p *printer) line() {
	p.out.WriteByte('\n')
	p.out.WriteString(strings.Repeat("  ", p.depth))
}

// member starts the next member of the innermost object or list on a line of its own.
func (p *printer) member() {
	if !p.bare {
		p.out.WriteByte(',')
	}
	p.bare = false
	p.line()
}

// key starts a member of an object: its name, for the value that is written next.
func (p *printer) key(name string) {
	p.member()
	p.out.WriteString(quoted(name))
	p.out.WriteString(": ")
}

// text writes a member that is a text. Only the quote, the backslash and the control characters are escaped:
// markup characters and characters outside ASCII are written as they are.
func (p *printer) text(key, value string) {
	p.key(key)
	p.out.WriteString(quoted(value))
}

func (p *printer) truth(key string, value bool) {
	p.key(key)
	if value {
		p.out.WriteString("true")
	} else {
		p.out.WriteString("false")
	}
}

// number writes a member that is a number, as encoding/json writes a float64. What is printed has one zero: the
// zero below 0, which a real can be, is written 0.
func (p *printer) number(key string, value float64) {
	p.key(key)
	if value == 0 {
		value = 0 // the zero below 0 equals 0, and this makes it the one that is written 0
	}
	written, err := json.Marshal(value)
	if err != nil {
		written = []byte("null") // JSON has no number that is not finite
	}
	p.out.Write(written)
}
