package manifest

import (
	"regexp"
	"slices"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/diag"
)

type ruleChecker struct {
	file string
	err  error
}

func (r *ruleChecker) check(holds bool, setting, rule string) {
	if r.err == nil && !holds {
		r.err = errBrokenRule(r.file, setting, rule)
	}
}

func (r *ruleChecker) checkText(text *string, setting string) {
	r.check(text == nil || !strings.Contains(*text, "\x00"), setting, "must not hold a NUL character")
}

func (r *ruleChecker) checkOneOf(text *string, allowed []string, setting string) {
	r.check(text == nil || slices.Contains(allowed, *text), setting, "must be one of "+diag.JoinWords(quoteAll(allowed), "or", -1))
}

func quoteAll(words []string) []string {
	quoted := make([]string, len(words))
	for index, word := range words {
		quoted[index] = `"` + word + `"`
	}
	return quoted
}

func isWithin[T int | int32 | float64](number *T, low, high T) bool {
	return number == nil || *number >= low && *number <= high
}

func isSetAndEmpty(text *string) bool {
	return text != nil && *text == ""
}

func pathSegments(path string) []string {
	var segments []string
	for _, segment := range strings.Split(strings.ReplaceAll(path, `\`, "/"), "/") {
		if segment != "" && segment != "." {
			segments = append(segments, segment)
		}
	}
	return segments
}

var driveLetter = regexp.MustCompile(`^[A-Za-z]:`)

func isRelativeDir(path string) bool {
	segments := pathSegments(path)
	return len(segments) > 0 && !strings.HasPrefix(path, "/") && !strings.HasPrefix(path, `\`) &&
		!driveLetter.MatchString(path) && !slices.Contains(segments, "..")
}

func isReservedDir(path string) bool {
	segments := pathSegments(strings.ToLower(path))
	if len(segments) == 0 {
		return false
	}
	return segments[0] == "maps" || segments[0] == "src" ||
		len(segments) >= 2 && segments[0] == "dist" && segments[1] == "stage"
}

var luaKeywords = []string{
	"and", "break", "do", "else", "elseif", "end", "false", "for", "function", "goto", "if", "in", "local", "nil", "not",
	"or", "repeat", "return", "then", "true", "until", "while",
}

var (
	identifier     = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	threeNumbers   = regexp.MustCompile(`^\d+\.\d+\.\d+$`)
	libraryName    = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
	gitHubRepoName = regexp.MustCompile(`^[A-Za-z0-9-]+/[A-Za-z0-9._-]+$`)
)

func isLuaName(name string) bool {
	return identifier.MatchString(name) && !slices.Contains(luaKeywords, name)
}

func isSingleLine(text string) bool {
	return !strings.ContainsAny(text, "\n\r\x00")
}

func errBrokenRule(file, setting, rule string) error {
	return &diag.Error{Msg: setting + " " + rule + ".", File: file, Hint: settingsListHint}
}
