package ini

import "strings"

type Section map[string]string

type File map[string]Section

func Parse(source string) File {
	file := File{}
	file.Add(source)
	return file
}

func (f File) Add(source string) {
	var section Section
	for _, raw := range strings.Split(source, "\n") {
		line := trim(raw)
		switch {
		case line == "" || strings.HasPrefix(line, "//"):
		case strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]"):
			section = f.sectionNamed(trim(line[1 : len(line)-1]))
		case section != nil:
			section.setFromLine(line)
		}
	}
}

func (f File) sectionNamed(name string) Section {
	if f[name] == nil {
		f[name] = Section{}
	}
	return f[name]
}

func (s Section) setFromLine(line string) {
	if key, value, found := strings.Cut(line, "="); found {
		s[trim(key)] = unquote(trim(value))
	}
}

func unquote(value string) string {
	if len(value) < 2 || value[0] != '"' || value[len(value)-1] != '"' {
		return value
	}
	if inner := value[1 : len(value)-1]; !strings.Contains(inner, `"`) {
		return inner
	}
	return value
}

func trim(text string) string { return strings.Trim(text, " \t\n\v\f\r") }
