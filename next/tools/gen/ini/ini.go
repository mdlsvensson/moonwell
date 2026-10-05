// Package ini reads the game's text files of sections, the editor's strings and the strings of each race:
// [section] lines, key=value lines and // comment lines.
//
// It takes the text of a file, decoded, and the sections read from the files before it; it returns those
// sections with the file's own added. It has no failure: a line that is none of the three is passed over.
//
// It must not know which sections and keys the generator reads, nor how a file is found and decoded: a byte order
// mark is text to it, and its white space is ASCII.
//
// It imports no package of the module.
package ini

import "strings"

// Section is a section's keys and their values.
type Section map[string]string

// File is the sections by name.
type File map[string]Section

// Parse adds the sections of source to into and returns it; a nil into starts a new file. A key that comes
// again, in this text or in a later one, has the later value. Lines before the first section are passed over.
func Parse(source string, into File) File {
	if into == nil {
		into = File{}
	}
	var section Section
	for _, raw := range strings.Split(strings.ReplaceAll(source, "\r\n", "\n"), "\n") {
		line := trim(raw)
		switch {
		case line == "" || strings.HasPrefix(line, "//"):
		case strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]"):
			section = into.section(trim(line[1 : len(line)-1]))
		case section != nil:
			section.set(line)
		}
	}
	return into
}

// section is the section of the name, which is added to the file when it has none.
func (f File) section(name string) Section {
	if f[name] == nil {
		f[name] = Section{}
	}
	return f[name]
}

// set reads a key=value line into the section; a line without an equals sign is passed over.
func (s Section) set(line string) {
	if key, value, found := strings.Cut(line, "="); found {
		s[trim(key)] = unquoted(trim(value))
	}
}

// unquoted is the value without its quotes when it is one quoted string: a quote at each end and none between
// them. A quoted list such as "a","b" is kept as it is written.
func unquoted(value string) string {
	if len(value) < 2 || value[0] != '"' || value[len(value)-1] != '"' {
		return value
	}
	if inner := value[1 : len(value)-1]; !strings.Contains(inner, `"`) {
		return inner
	}
	return value
}

// trim takes the ASCII white space off both ends of text.
func trim(text string) string { return strings.Trim(text, " \t\n\v\f\r") }
