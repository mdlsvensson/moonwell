package main

import (
	"strings"
	"testing"
)

var handWritten = map[string]func(text string) error{
	extrasPath: func(text string) error {
		_, err := decodeExtras([]byte(text))
		return err
	},
	overridesPath: func(text string) error {
		_, err := decodeOverrides([]byte(text))
		return err
	},
}

func TestAHandWrittenFileMayStartWithAByteOrderMark(t *testing.T) {
	for path, decode := range handWritten {
		if err := decode("\xEF\xBB\xBF{}"); err != nil {
			t.Errorf("%s with a byte order mark: %v", path, err)
		}
		if err := decode("{}\xEF\xBB\xBF"); err == nil || !strings.HasPrefix(err.Error(), path+": ") {
			t.Errorf("%s with a byte order mark at its end: got %v, want the file refused by its path", path, err)
		}
	}
}

func TestAHandWrittenFileIsRefusedForAKeyThatStandsTwice(t *testing.T) {
	for _, c := range []struct{ path, text, key string }{
		{extrasPath, `{"globals": ["a"], "removed": ["b"], "globals": ["c"]}`, "globals"},
		{extrasPath, `{"functions": [{"name": "A", "params": [], "returns": "b", "name": "B"}]}`, "name"},
		{extrasPath, `{"functions": [{"name": "A", "params": [{"type": "b", "name": "c", "type": "d"}], ` +
			`"returns": "e"}]}`, "type"},
		{overridesPath, `{"names": {}, "removed": {}, "names": {"units": {"ucls": "unitClass"}}}`, "names"},
		{overridesPath, `{"names": {"units": {"ucls": "a"}, "units": {"uhpm": "b"}}}`, "units"},
		{overridesPath, `{"names": {"units": {"ucls": "a", "uhpm": "b", "ucls": "c"}}}`, "ucls"},
		{overridesPath, `{"removed": {"buffs": [], "buffs": ["fold"]}}`, "buffs"},
	} {
		err := handWritten[c.path](c.text)
		if err == nil {
			t.Errorf("%s: %s was read", c.path, c.text)
			continue
		}
		checkContains(t, err.Error(), c.path+": ", `"`+c.key+`"`, "twice")
	}
	for path, text := range map[string]string{
		extrasPath: `{"functions": [{"name": "A", "params": [{"name": "a", "type": "b"}, {"name": "c", "type": "d"}], ` +
			`"returns": "e"}, {"name": "B", "params": [], "returns": "e"}]}`,
		overridesPath: `{"names": {"units": {"ucls": "a"}, "items": {"ucls": "a"}}, "removed": {"units": ["ucls"]}}`,
	} {
		if err := handWritten[path](text); err != nil {
			t.Errorf("%s: %s: %v", path, text, err)
		}
	}
}

func TestAHandWrittenFileIsRefusedForAKeyThatTheGeneratorDoesNotRead(t *testing.T) {
	for _, c := range []struct{ path, text, key string }{
		{extrasPath, `{"functions": [], "more": 1}`, "more"},
		{extrasPath, `{"functions": [{"name": "A", "params": [], "returns": "b", "constant": true}]}`, "constant"},
		{extrasPath, `{"functions": [{"name": "A", "params": [{"name": "a", "type": "b", "array": 0}], ` +
			`"returns": "b"}]}`, "array"},
		{overridesPath, `{"names": {}, "renamed": {}}`, "renamed"},
		{overridesPath, `{"zzz": 1e999}`, "zzz"},
	} {
		err := handWritten[c.path](c.text)
		if err == nil {
			t.Errorf("%s: %s was read", c.path, c.text)
			continue
		}
		checkContains(t, err.Error(), c.path+": ", `unknown field "`+c.key+`"`)
	}
}

func TestAHandWrittenFileIsRefusedForATextThatDoesNotFit(t *testing.T) {
	for _, c := range []struct{ path, text, words string }{
		{extrasPath, `{"functions": [{"name": "Fo`, "unexpected EOF"},
		{extrasPath, ``, "the file is empty"},
		{extrasPath, "not JSON\n", "invalid character 'o'"},
		{extrasPath, `{"globals": ["print",]}`, "invalid character ']'"},
		{extrasPath, `{} {}`, "something follows the JSON value"},
		{extrasPath, `[]`, "the file is of the wrong kind (array)"},
		{extrasPath, `"functions"`, "the file is of the wrong kind (string)"},
		{extrasPath, `{"globals": "print"}`, "globals is of the wrong kind (string)"},
		{extrasPath, `{"removed": ["io", 1]}`, "removed.1 is of the wrong kind (number)"},
		{extrasPath, `{"functions": {"name": "A"}}`, "functions is of the wrong kind (object)"},
		{extrasPath, `{"functions": [{"name": 1, "params": [], "returns": "b"}]}`, "functions.0.name is of the wrong kind"},
		{extrasPath, `{"functions": [{"name": "A", "params": [{"name": "a", "type": 1}], "returns": "b"}]}`,
			"functions.0.params.0.type is of the wrong kind (number)"},
		{extrasPath, `{"functions": [{"name": "A", "params": ["a"], "returns": "b"}]}`,
			"functions.0.params.0 is of the wrong kind (string)"},
		{overridesPath, `{"names": {"units": {"ucls": "unitCl`, "unexpected EOF"},
		{overridesPath, ``, "the file is empty"},
		{overridesPath, "not JSON\n", "invalid character 'o'"},
		{overridesPath, `{} {}`, "something follows the JSON value"},
		{overridesPath, `[]`, "the file is of the wrong kind (array)"},
		{overridesPath, `{"names": "ucls"}`, "names is of the wrong kind (string)"},
		{overridesPath, `{"names": {"units": {"ucls": 1}}}`, "names.units.ucls is of the wrong kind (number)"},
		{overridesPath, `{"names": {"units": ["ucls"]}}`, "names.units is of the wrong kind (array)"},
		{overridesPath, `{"removed": {"units": "uold"}}`, "removed.units is of the wrong kind (string)"},
	} {
		err := handWritten[c.path](c.text)
		if err == nil {
			t.Errorf("%s: %s was read", c.path, c.text)
			continue
		}
		checkContains(t, err.Error(), c.path+": ", c.words)
		for _, ofGo := range []string{"Go ", "unmarshal", "struct", "main."} {
			if strings.Contains(err.Error(), ofGo) {
				t.Errorf("%s: %s: the refusal has %q in it: %v", c.path, c.text, ofGo, err)
			}
		}
	}
}
