package objects

import (
	"bytes"
	"encoding/json"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/manifest"
	"github.com/mdlsvensson/moonwell/internal/war3/objmod"
)

func EvalJSON(resolved []Resolved) []byte {
	var p jsonWriter
	p.openBracket('{')
	for _, category := range manifest.Categories {
		p.writeKey(string(category))
		p.openBracket('{')
		for _, object := range resolved {
			if object.Category == category {
				p.writeKey(object.Key)
				p.writeObject(object)
			}
		}
		p.closeBracket('}')
	}
	p.closeBracket('}')
	return p.out.Bytes()
}

func (p *jsonWriter) writeObject(object Resolved) {
	p.openBracket('{')
	p.writeString("id", object.ID)
	p.writeString("base", object.Base)
	p.writeString("source", object.Source)
	p.writeKey("fields")
	p.openBracket('[')
	for _, field := range object.Fields {
		p.startMember()
		p.writeField(field)
	}
	p.closeBracket(']')
	p.closeBracket('}')
}

var typeNames = map[objmod.ValueType]string{
	objmod.Int: "int", objmod.Real: "real", objmod.Unreal: "unreal", objmod.String: "string",
}

func (p *jsonWriter) writeField(field Field) {
	p.openBracket('{')
	p.writeString("rawcode", field.ID)
	p.writeString("name", field.Name)
	p.writeNumber("level", float64(field.Level))
	p.writeNumber("column", float64(field.Column))
	p.writeBool("skin", field.Skin)
	p.writeString("type", typeNames[field.Value.Type])
	if field.Value.Type == objmod.String {
		p.writeString("value", field.Value.Text)
	} else {
		p.writeNumber("value", field.Value.Number)
	}
	p.closeBracket('}')
}

type jsonWriter struct {
	out   bytes.Buffer
	depth int
	bare  bool
}

func (p *jsonWriter) openBracket(bracket byte) {
	p.out.WriteByte(bracket)
	p.depth++
	p.bare = true
}

func (p *jsonWriter) closeBracket(bracket byte) {
	p.depth--
	if !p.bare {
		p.newLine()
	}
	p.out.WriteByte(bracket)
	p.bare = false
}

func (p *jsonWriter) newLine() {
	p.out.WriteByte('\n')
	p.out.WriteString(strings.Repeat("  ", p.depth))
}

func (p *jsonWriter) startMember() {
	if !p.bare {
		p.out.WriteByte(',')
	}
	p.bare = false
	p.newLine()
}

func (p *jsonWriter) writeKey(name string) {
	p.startMember()
	p.out.WriteString(fsx.QuoteJSON(name))
	p.out.WriteString(": ")
}

func (p *jsonWriter) writeString(key, value string) {
	p.writeKey(key)
	p.out.WriteString(fsx.QuoteJSON(value))
}

func (p *jsonWriter) writeBool(key string, value bool) {
	p.writeKey(key)
	if value {
		p.out.WriteString("true")
	} else {
		p.out.WriteString("false")
	}
}

func (p *jsonWriter) writeNumber(key string, value float64) {
	p.writeKey(key)
	if value == 0 {
		value = 0
	}
	written, err := json.Marshal(value)
	if err != nil {
		written = []byte("null")
	}
	p.out.Write(written)
}
