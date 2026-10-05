// Package ini reads the game's text files of sections, the editor's strings and the strings of each race:
// [section] lines, key=value lines and // comment lines.
//
// It takes the text of a file, decoded, and returns its sections, or adds them to the sections read from the
// files before it. It has no failure: a line that is none of the three is passed over.
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

// Parse reads the sections of a text.
func Parse(source string) File {
	file := File{}
	file.Add(source)
	return file
}

// Add adds the sections of source to the file, which Parse made or which is File{}: a section the file has gains
// the text's keys, and a key given again, in this text or in one added before, takes the later value. A text
// starts outside every section, so its lines before the first [section] line are passed over.
func (f File) Add(source string) {
	var section Section
	for _, raw := range strings.Split(strings.ReplaceAll(source, "\r\n", "\n"), "\n") {
		line := trim(raw)
		switch {
		case line == "" || strings.HasPrefix(line, "//"):
		case strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]"):
			section = f.section(trim(line[1 : len(line)-1]))
		case section != nil:
			section.set(line)
		}
	}
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
