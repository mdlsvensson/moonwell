package editor

import (
	"bytes"
	"encoding/json"
	"errors"

	moonwell "github.com/mdlsvensson/moonwell"
	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/manifest"
)

var luarcArrays = []string{"runtime.path", "workspace.library", "workspace.ignoreDir"}

func MergeLuarc(root string, template []moonwell.TemplateFile) (added []string, merged bool, err error) {
	data, found, err := readFileIfExists(root, luarcFile)
	switch {
	case err != nil:
		return nil, false, err
	case !found:
		return []string{}, true, nil
	}
	entries, err := LuarcTemplateEntries(template)
	if err != nil {
		return nil, false, err
	}
	config, isObject := asObject(data)
	if !isObject {
		return nil, false, nil
	}
	added = addMissingEntries(&config, entries)
	if len(added) == 0 {
		return added, true, nil
	}
	if err := writeLuarc(root, config); err != nil {
		return nil, false, err
	}
	return added, true, nil
}

func addMissingEntries(config *manifest.OrderedMap[json.RawMessage], entries map[string][]string) []string {
	added := []string{}
	for _, key := range luarcArrays {
		added = append(added, addEntries(config, key, entries[key])...)
	}
	return added
}

func LuarcTemplateEntries(template []moonwell.TemplateFile) (map[string][]string, error) {
	data, err := templateFile(template, luarcFile)
	if err != nil {
		return nil, err
	}
	config, isObject := asObject(data)
	if !isObject {
		return nil, errors.New("editor: the template's .luarc.json is not a JSON object")
	}
	entries := map[string][]string{}
	for _, key := range luarcArrays {
		raw, _ := config.Get(key)
		list, isList := asStringList(raw)
		if !isList {
			return nil, errors.New("editor: the template's .luarc.json has no " + key + " array of strings")
		}
		entries[key] = list
	}
	return entries, nil
}

func addEntries(config *manifest.OrderedMap[json.RawMessage], key string, entries []string) (added []string) {
	raw, exists := config.Get(key)
	elements, isArray := asArray(raw)
	if exists && !isArray {
		return nil
	}
	existing := map[string]bool{}
	for _, element := range elements {
		if text, isString := asString(element); isString {
			existing[text] = true
		}
	}
	for _, entry := range entries {
		if existing[entry] {
			continue
		}
		existing[entry] = true
		elements = append(elements, json.RawMessage(fsx.QuoteJSON(entry)))
		added = append(added, entry)
	}
	config.Set(key, toJSONArray(elements))
	return added
}

func writeLuarc(root string, config manifest.OrderedMap[json.RawMessage]) error {
	text, err := formatLuarc(config)
	if err != nil {
		return err
	}
	return writeFile(root, luarcFile, text)
}

func formatLuarc(config manifest.OrderedMap[json.RawMessage]) ([]byte, error) {
	var compact bytes.Buffer
	compact.WriteByte('{')
	for key, value := range config.All() {
		if compact.Len() > 1 {
			compact.WriteByte(',')
		}
		compact.WriteString(fsx.QuoteJSON(key))
		compact.WriteByte(':')
		compact.Write(value)
	}
	compact.WriteByte('}')
	var text bytes.Buffer
	if err := json.Indent(&text, compact.Bytes(), "", "  "); err != nil {
		return nil, err
	}
	text.WriteByte('\n')
	return text.Bytes(), nil
}

func asObject(text []byte) (config manifest.OrderedMap[json.RawMessage], isObject bool) {
	text = fsx.TrimBOM(text)
	if !startsWith(text, '{') || json.Unmarshal(text, &config) != nil {
		return manifest.OrderedMap[json.RawMessage]{}, false
	}
	return config, true
}

func asArray(raw json.RawMessage) (elements []json.RawMessage, isArray bool) {
	if !startsWith(raw, '[') || json.Unmarshal(raw, &elements) != nil {
		return nil, false
	}
	return elements, true
}

func asString(raw json.RawMessage) (text string, isString bool) {
	if !startsWith(raw, '"') || json.Unmarshal(raw, &text) != nil {
		return "", false
	}
	return text, true
}

func asStringList(raw json.RawMessage) (list []string, isList bool) {
	elements, isArray := asArray(raw)
	if !isArray {
		return nil, false
	}
	for _, element := range elements {
		text, isString := asString(element)
		if !isString {
			return nil, false
		}
		list = append(list, text)
	}
	return list, true
}

func startsWith(text []byte, first byte) bool {
	text = bytes.TrimLeft(text, jsonSpace)
	return len(text) > 0 && text[0] == first
}

func toJSONArray(elements []json.RawMessage) json.RawMessage {
	array := json.RawMessage("[")
	for i, element := range elements {
		if i > 0 {
			array = append(array, ',')
		}
		array = append(array, element...)
	}
	return append(array, ']')
}
