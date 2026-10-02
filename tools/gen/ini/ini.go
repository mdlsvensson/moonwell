// Package ini reads the game's INI-style .txt files (WorldEditStrings.txt, the per-race strings files): [section]
// headers, Key=value lines and // comment lines.
package ini

import (
	"strings"

	"github.com/mdlsvensson/moonwell/internal/ordered"
	"github.com/mdlsvensson/moonwell/internal/text"
)

// bom is the byte order mark some of the game's text files start with.
const bom = "\xEF\xBB\xBF"

// Section maps a section's keys to their values.
type Section = ordered.Map[string]

// File maps section names to their sections.
type File = ordered.Map[*Section]

// Parse adds the sections of source to into; nil starts a new file. A repeated key, in the same or a later text,
// replaces the earlier value. A value that is one quoted string is unquoted; a quoted list such as "a","b" is kept
// as written. Lines before the first section are ignored.
func Parse(source string, into *File) *File {
	if into == nil {
		into = &File{}
	}
	var section *Section
	source = strings.TrimPrefix(source, bom)
	for _, raw := range strings.Split(strings.ReplaceAll(source, "\r\n", "\n"), "\n") {
		line := text.Trim(raw)
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			name := text.Trim(line[1 : len(line)-1])
			found, ok := into.Get(name)
			if !ok {
				found = &Section{}
				into.Set(name, found)
			}
			section = found
			continue
		}
		equals := strings.IndexByte(line, '=')
		if section == nil || equals < 0 {
			continue
		}
		value := text.Trim(line[equals+1:])
		if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' && !strings.Contains(value[1:len(value)-1], `"`) {
			value = value[1 : len(value)-1]
		}
		section.Set(text.Trim(line[:equals]), value)
	}
	return into
}
