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
	text := takeApart(source)
	for _, section := range sections {
		for _, field := range section.Fields {
			text.lines = set(text.lines, section.Name, field)
		}
	}
	return text.joined()
}

type document struct {
	lines        []string
	newline      string
	finalNewline bool
}

func takeApart(source string) document {
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

func (d document) joined() string {
	text := strings.Join(d.lines, d.newline)
	if d.finalNewline {
		text += d.newline
	}
	return text
}

const whiteSpace = fsx.ASCIISpace

var (
	header  = regexp.MustCompile(`^[` + whiteSpace + `]*\[([^\]]+)\][` + whiteSpace + `]*(?:(?://|;)[^\r\n]*)?$`)
	setting = regexp.MustCompile(`^([` + whiteSpace + `]*)([^=` + whiteSpace + `]+)[` + whiteSpace + `]*=`)
	blank   = regexp.MustCompile(`^[` + whiteSpace + `]*$`)
)

func set(lines []string, section string, field Field) []string {
	inside := linesOf(lines, section)
	if replace(lines, inside, field) {
		return lines
	}
	if len(inside) == 0 {
		return appendSection(lines, section, field)
	}
	return slices.Insert(lines, end(lines, inside), line(field))
}

func linesOf(lines []string, section string) []int {
	var inside []int
	open := false
	for i, text := range lines {
		if name := header.FindStringSubmatch(text); name != nil {
			open = strings.EqualFold(name[1], section)
		}
		if open {
			inside = append(inside, i)
		}
	}
	return inside
}

func replace(lines []string, inside []int, field Field) bool {
	found := false
	for _, i := range inside {
		if parts := setting.FindStringSubmatch(lines[i]); parts != nil && strings.EqualFold(parts[2], field.Key) {
			lines[i] = parts[1] + parts[2] + "=" + field.Value
			found = true
		}
	}
	return found
}

func end(lines []string, inside []int) int {
	for _, i := range slices.Backward(inside) {
		if !blank.MatchString(lines[i]) {
			return i + 1
		}
	}
	return len(lines)
}

func appendSection(lines []string, section string, field Field) []string {
	if len(lines) > 0 && !blank.MatchString(lines[len(lines)-1]) {
		lines = append(lines, "")
	}
	return append(lines, "["+section+"]", line(field))
}

func line(field Field) string { return field.Key + "=" + field.Value }
