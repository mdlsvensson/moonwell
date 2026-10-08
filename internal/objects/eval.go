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

var typeNames = map[objmod.ValueType]string{
	objmod.Int: "int", objmod.Real: "real", objmod.Unreal: "unreal", objmod.String: "string",
}

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

type printer struct {
	out   bytes.Buffer
	depth int
	bare  bool
}

func (p *printer) open(bracket byte) {
	p.out.WriteByte(bracket)
	p.depth++
	p.bare = true
}

func (p *printer) close(bracket byte) {
	p.depth--
	if !p.bare {
		p.line()
	}
	p.out.WriteByte(bracket)
	p.bare = false
}

func (p *printer) line() {
	p.out.WriteByte('\n')
	p.out.WriteString(strings.Repeat("  ", p.depth))
}

func (p *printer) member() {
	if !p.bare {
		p.out.WriteByte(',')
	}
	p.bare = false
	p.line()
}

func (p *printer) key(name string) {
	p.member()
	p.out.WriteString(fsx.QuoteJSON(name))
	p.out.WriteString(": ")
}

func (p *printer) text(key, value string) {
	p.key(key)
	p.out.WriteString(fsx.QuoteJSON(value))
}

func (p *printer) truth(key string, value bool) {
	p.key(key)
	if value {
		p.out.WriteString("true")
	} else {
		p.out.WriteString("false")
	}
}

func (p *printer) number(key string, value float64) {
	p.key(key)
	if value == 0 {
		value = 0
	}
	written, err := json.Marshal(value)
	if err != nil {
		written = []byte("null")
	}
	p.out.Write(written)
}
