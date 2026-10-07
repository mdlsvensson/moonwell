// Package txt merges settings into the section-and-key text files of a map (war3mapMisc.txt, war3mapSkin.txt)
// without disturbing what else they hold.
//
// Merge takes the text of such a file, decoded and without a byte order mark, and the sections to set, each with
// its fields in the order to apply them. It returns the new text. A file is lines: a header `[Name]` opens a
// section, a line `Key=Value` sets a key in the section it stands in, and every other line is kept as it is.
// White space is ASCII white space throughout: a space, a tab, a vertical tab, a form feed, a carriage return, a
// line feed.
//
// The package must not know which file a text belongs to, how the file is read or written, or which settings
// Moonwell sets: names and values are written as they are given.
package txt

import (
	"regexp"
	"slices"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/fsx"
)

// Field is one key and the value to give it.
type Field struct{ Key, Value string }

// Section is the fields to set in the section of one name.
type Section struct {
	Name   string
	Fields []Field
}

// Merge sets each field in source. Names match without regard to letter case. A new key goes after the last
// non-blank line of its section, a new section after one blank line. The newline style and the final newline of
// source are kept.
//
// The fields are applied one after the other, each to the lines the one before it left. A key that several lines
// of its sections set is given its value in every one of them; a key that none sets goes to the last section of
// the name. A source that has both newline styles comes back with CRLF.
func Merge(source string, sections []Section) string {
	text := takeApart(source)
	for _, section := range sections {
		for _, field := range section.Fields {
			text.lines = set(text.lines, section.Name, field)
		}
	}
	return text.joined()
}

// document is a text taken apart into its lines, with what it takes to put it together again.
type document struct {
	lines        []string
	newline      string // "\r\n" when a line of the source ends with it, else "\n"
	finalNewline bool   // the source ends with a line break
}

// takeApart splits source at each line feed and at each carriage return and line feed. An empty source has no
// lines.
func takeApart(source string) document {
	text := document{newline: "\n", finalNewline: strings.HasSuffix(source, "\n")}
	if strings.Contains(source, "\r\n") {
		text.newline = "\r\n"
	}
	if source != "" {
		text.lines = strings.Split(strings.ReplaceAll(source, "\r\n", "\n"), "\n")
	}
	if text.finalNewline {
		text.lines = text.lines[:len(text.lines)-1] // the empty piece after the last line break
	}
	return text
}

// joined is the lines as one text again.
func (d document) joined() string {
	text := strings.Join(d.lines, d.newline)
	if d.finalNewline {
		text += d.newline
	}
	return text
}

// whiteSpace is the six white space characters of ASCII, as the inside of a character class. The `\s` of regexp
// has five of them: it leaves the vertical tab out.
const whiteSpace = fsx.ASCIISpace

var (
	// header matches a line that opens a section and captures the section's name. Only white space and a comment
	// that starts with `//` or `;` may stand beside the brackets. The comment holds no carriage return. A carriage
	// return left inside a line ended a line for whoever wrote it, so what follows it may be another line of the
	// file and not comment; such a line is not taken for a header, and nothing is merged into a section whose
	// lines cannot be told apart.
	header = regexp.MustCompile(`^[` + whiteSpace + `]*\[([^\]]+)\][` + whiteSpace + `]*(?:(?://|;)[^\r\n]*)?$`)
	// setting matches the start of a line that sets a key, and captures the line's indent and the key.
	setting = regexp.MustCompile(`^([` + whiteSpace + `]*)([^=` + whiteSpace + `]+)[` + whiteSpace + `]*=`)
	// blank matches a line of nothing but white space.
	blank = regexp.MustCompile(`^[` + whiteSpace + `]*$`)
)

// set gives a field its value in the section: in the lines that set its key, or else on a line of its own.
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

// linesOf returns the indexes of the lines of every section of the name, each section's header among them.
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

// replace gives the field's value to each of the lines that sets its key. Such a line keeps its indent and its
// own spelling of the key; whatever followed the key gives way to the value. It reports whether there was such a
// line.
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

// end is the index after the last of a section's lines that is not blank. A header is never blank, so the lines
// of a section always have one.
func end(lines []string, inside []int) int {
	for _, i := range slices.Backward(inside) {
		if !blank.MatchString(lines[i]) {
			return i + 1
		}
	}
	return len(lines)
}

// appendSection adds a section with the one field at the end of the lines, after one blank line. There is no
// blank line before the first line of a text, and no second one after a line that is blank already.
func appendSection(lines []string, section string, field Field) []string {
	if len(lines) > 0 && !blank.MatchString(lines[len(lines)-1]) {
		lines = append(lines, "")
	}
	return append(lines, "["+section+"]", line(field))
}

// line is the line that sets a field.
func line(field Field) string { return field.Key + "=" + field.Value }
