package txt

import (
	"regexp"
	"slices"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/fsx"
)

type Field struct{ Key, Value string }

type Section struct {
	Name   string
	Fields []Field
}

func Merge(source string, sections []Section) string {
	text := splitLines(source)
	for _, section := range sections {
		for _, field := range section.Fields {
			text.lines = setField(text.lines, section.Name, field)
		}
	}
	return text.join()
}

type document struct {
	lines        []string
	newline      string
	finalNewline bool
}

func splitLines(source string) document {
	text := document{newline: "\n", finalNewline: strings.HasSuffix(source, "\n")}
	if strings.Contains(source, "\r\n") {
		text.newline = "\r\n"
	}
	if source != "" {
		text.lines = strings.Split(strings.ReplaceAll(source, "\r\n", "\n"), "\n")
	}
	if text.finalNewline {
		text.lines = text.lines[:len(text.lines)-1]
	}
	return text
}

func (d document) join() string {
	text := strings.Join(d.lines, d.newline)
	if d.finalNewline {
		text += d.newline
	}
	return text
}

const whiteSpace = fsx.ASCIISpace

var (
	headerPattern = regexp.MustCompile(`^[` + whiteSpace + `]*\[([^\]]+)\][` + whiteSpace + `]*(?:(?://|;)[^\r\n]*)?$`)
	fieldPattern  = regexp.MustCompile(`^([` + whiteSpace + `]*)([^=` + whiteSpace + `]+)[` + whiteSpace + `]*=`)
	blankPattern  = regexp.MustCompile(`^[` + whiteSpace + `]*$`)
)

func setField(lines []string, section string, field Field) []string {
	indexes := sectionLineIndexes(lines, section)
	if replaceField(lines, indexes, field) {
		return lines
	}
	if len(indexes) == 0 {
		return appendSection(lines, section, field)
	}
	return slices.Insert(lines, sectionEnd(lines, indexes), formatField(field))
}

func sectionLineIndexes(lines []string, section string) []int {
	var indexes []int
	inSection := false
	for i, text := range lines {
		if name := headerPattern.FindStringSubmatch(text); name != nil {
			inSection = strings.EqualFold(name[1], section)
		}
		if inSection {
			indexes = append(indexes, i)
		}
	}
	return indexes
}

func replaceField(lines []string, indexes []int, field Field) bool {
	found := false
	for _, i := range indexes {
		if parts := fieldPattern.FindStringSubmatch(lines[i]); parts != nil && strings.EqualFold(parts[2], field.Key) {
			lines[i] = parts[1] + parts[2] + "=" + field.Value
			found = true
		}
	}
	return found
}

func sectionEnd(lines []string, indexes []int) int {
	for _, i := range slices.Backward(indexes) {
		if !blankPattern.MatchString(lines[i]) {
			return i + 1
		}
	}
	return len(lines)
}

func appendSection(lines []string, section string, field Field) []string {
	if len(lines) > 0 && !blankPattern.MatchString(lines[len(lines)-1]) {
		lines = append(lines, "")
	}
	return append(lines, "["+section+"]", formatField(field))
}

func formatField(field Field) string { return field.Key + "=" + field.Value }
