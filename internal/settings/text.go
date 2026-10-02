package settings

import (
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/ordered"
	"github.com/mdlsvensson/moonwell/internal/text"
)

var (
	lineBreak     = regexp.MustCompile(`\r?\n`)
	sectionHeader = regexp.MustCompile(`^` + text.SpaceClass + `*\[([^\]]+)\]` + text.SpaceClass + `*(?:(?://|;)` + text.NotLineBreak + `*)?$`)
	fieldLine     = regexp.MustCompile(`^(` + text.SpaceClass + `*)([^=` + text.SpaceSet + `]+)` + text.SpaceClass + `*=`)
)

// PatchText merges raw section, key and value text into a Warcraft settings file without discarding unrelated
// editor settings. A new key goes after the last non-blank line of its (last matching) section; a new section is
// appended after one blank line. The source's newline style and final newline are kept.
func PatchText(source string, sections Sections) string {
	newline := "\n"
	if strings.Contains(source, "\r\n") {
		newline = "\r\n"
	}
	var lines []string
	if source != "" {
		lines = lineBreak.Split(source, -1)
	}
	finalNewline := len(lines) > 1 && lines[len(lines)-1] == ""
	if finalNewline {
		lines = lines[:len(lines)-1]
	}
	for section, entries := range sections.All() {
		for key, value := range entries.All() {
			active, found := false, false
			insertion := -1
			for i, line := range lines {
				if header := sectionHeader.FindStringSubmatch(line); header != nil {
					active = text.Lower(header[1]) == text.Lower(section)
				}
				if !active {
					continue
				}
				if text.Trim(line) != "" {
					insertion = i + 1
				}
				if field := fieldLine.FindStringSubmatch(line); field != nil && text.Lower(field[2]) == text.Lower(key) {
					lines[i] = field[1] + field[2] + "=" + value
					found = true
				}
			}
			if found {
				continue
			}
			if insertion == -1 {
				if len(lines) > 0 && text.Trim(lines[len(lines)-1]) != "" {
					lines = append(lines, "")
				}
				lines = append(lines, "["+section+"]", key+"="+value)
			} else {
				lines = slices.Insert(lines, insertion, key+"="+value)
			}
		}
	}
	patched := strings.Join(lines, newline)
	if finalNewline {
		patched += newline
	}
	return patched
}

// GameplaySections returns a fresh merge of the raw gameplay constants and the typed ones (heroMaxLevel,
// foodLimit). A typed value that disagrees with a raw one fails, naming file.
func GameplaySections(s *Settings, file string) (Sections, error) {
	var sections Sections
	for name, fields := range s.GameplayConstants.All() {
		copied := &ordered.Map[string]{}
		for key, value := range fields.All() {
			copied.Set(key, value)
		}
		sections.Set(name, copied)
	}
	for _, typed := range []struct {
		name, raw string
		value     *int
	}{
		{"heroMaxLevel", "MaxHeroLevel", s.Gameplay.HeroMaxLevel},
		{"foodLimit", "FoodCeiling", s.Gameplay.FoodLimit},
	} {
		if typed.value == nil {
			continue
		}
		value := strconv.Itoa(*typed.value)
		section := "Misc"
		for _, name := range sections.Keys() {
			if strings.ToLower(name) == "misc" {
				section = name
				break
			}
		}
		fields, ok := sections.Get(section)
		if !ok {
			fields = &ordered.Map[string]{}
		}
		key := typed.raw
		for _, name := range fields.Keys() {
			if strings.ToLower(name) == strings.ToLower(typed.raw) {
				key = name
				break
			}
		}
		if existing, ok := fields.Get(key); ok && existing != value {
			return Sections{}, &diag.Error{
				Msg:  "Conflicting typed and raw gameplay constant: " + typed.raw + ".",
				File: file,
				Hint: "Remove the raw " + typed.raw + " override or make it equal to settings.gameplay." + typed.name + ".",
			}
		}
		fields.Set(key, value)
		sections.Set(section, fields)
	}
	return sections, nil
}
