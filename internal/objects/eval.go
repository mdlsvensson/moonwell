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
	var w jsonWriter
	w.openBracket('{')
	for _, category := range manifest.Categories {
		w.writeKey(string(category))
		w.openBracket('{')
		for _, object := range resolved {
			if object.Category == category {
				w.writeKey(object.Key)
				w.writeObject(object)
			}
		}
		w.closeBracket('}')
	}
	w.closeBracket('}')
	return w.out.Bytes()
}

type jsonWriter struct {
	out     bytes.Buffer
	depth   int
	isEmpty bool
}

func (w *jsonWriter) writeObject(object Resolved) {
	w.openBracket('{')
	w.writeString("id", object.ID)
	w.writeString("base", object.Base)
	w.writeString("source", object.Source)
	w.writeKey("fields")
	w.openBracket('[')
	for _, field := range object.Fields {
		w.startMember()
		w.writeField(field)
	}
	w.closeBracket(']')
	w.closeBracket('}')
}

var typeNames = map[objmod.ValueType]string{
	objmod.Int: "int", objmod.Real: "real", objmod.Unreal: "unreal", objmod.String: "string",
}

func (w *jsonWriter) writeField(field Field) {
	w.openBracket('{')
	w.writeString("rawcode", field.ID)
	w.writeString("name", field.Name)
	w.writeNumber("level", float64(field.Level))
	w.writeNumber("column", float64(field.Column))
	w.writeBool("skin", field.Skin)
	w.writeString("type", typeNames[field.Value.Type])
	if field.Value.Type == objmod.String {
		w.writeString("value", field.Value.Text)
	} else {
		w.writeNumber("value", field.Value.Number)
	}
	w.closeBracket('}')
}

func (w *jsonWriter) openBracket(bracket byte) {
	w.out.WriteByte(bracket)
	w.depth++
	w.isEmpty = true
}

func (w *jsonWriter) closeBracket(bracket byte) {
	w.depth--
	if !w.isEmpty {
		w.newLine()
	}
	w.out.WriteByte(bracket)
	w.isEmpty = false
}

func (w *jsonWriter) newLine() {
	w.out.WriteByte('\n')
	w.out.WriteString(strings.Repeat("  ", w.depth))
}

func (w *jsonWriter) startMember() {
	if !w.isEmpty {
		w.out.WriteByte(',')
	}
	w.isEmpty = false
	w.newLine()
}

func (w *jsonWriter) writeKey(name string) {
	w.startMember()
	w.out.WriteString(fsx.QuoteJSON(name))
	w.out.WriteString(": ")
}

func (w *jsonWriter) writeString(key, value string) {
	w.writeKey(key)
	w.out.WriteString(fsx.QuoteJSON(value))
}

func (w *jsonWriter) writeBool(key string, value bool) {
	w.writeKey(key)
	if value {
		w.out.WriteString("true")
	} else {
		w.out.WriteString("false")
	}
}

func (w *jsonWriter) writeNumber(key string, value float64) {
	w.writeKey(key)
	written, err := json.Marshal(withoutNegativeZero(value))
	if err != nil {
		written = []byte("null")
	}
	w.out.Write(written)
}

func withoutNegativeZero(value float64) float64 {
	if value == 0 {
		return 0
	}
	return value
}
