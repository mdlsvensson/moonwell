package moonwell

import (
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"
)

func TestTheDocumentsNameFilesAndFoldersThatAreThere(t *testing.T) {
	folders := []string{".github", "cmd", "data", "internal", "runtime", "schema", "template", "tools"}
	const ellipsis = "\xE2\x80\xA6"
	aFile := regexp.MustCompile(`\.(go|md|mod|ps1|sh)$`)
	backticked := regexp.MustCompile("`([^`]+)`")
	for _, document := range []string{"ARCHITECTURE.md", "CONTRIBUTING.md"} {
		text, err := os.ReadFile(document)
		if err != nil {
			t.Fatal(err)
		}
		inCode := false
		for number, line := range strings.Split(string(text), "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "```") {
				inCode = !inCode
			}
			if inCode {
				continue
			}
			for _, match := range backticked.FindAllStringSubmatch(line, -1) {
				path := match[1]
				first, _, _ := strings.Cut(path, "/")
				named := slices.Contains(folders, first) || aFile.MatchString(path)
				if !named || strings.ContainsAny(path, " <>*("+ellipsis) {
					continue
				}
				if _, err := os.Stat(path); err != nil {
					t.Errorf("%s:%d names `%s`, which is not in the repository", document, number+1, path)
				}
			}
		}
	}
}

func TestArchitectureQuotesPlanAsTheSourceHasIt(t *testing.T) {
	document, _ := os.ReadFile("ARCHITECTURE.md")
	source, _ := os.ReadFile("internal/build/build.go")
	_, quote, _ := strings.Cut(string(document), "```go\nfunc Plan(")
	quote, _, _ = strings.Cut(quote, "\n```")
	_, body, _ := strings.Cut(string(source), "\nfunc Plan(")
	body, _, _ = strings.Cut(body, "\n}\n")
	leftOut := func(line string) bool {
		return slices.Contains([]string{"", "if err != nil {", "return nil, err", "}"}, strings.TrimSpace(line))
	}
	quoted := slices.DeleteFunc(strings.Split(quote, "\n"), leftOut)
	written := slices.DeleteFunc(strings.Split(body, "\n"), leftOut)
	if len(written) == 0 || !slices.Equal(quoted, written) {
		t.Errorf("Plan changed, or its quote did: update the quote of Plan in ARCHITECTURE.md, and the numbered "+
			"steps below it, to say what internal/build/build.go has.\nThe document quotes:\n%s\nThe source has:\n%s",
			strings.Join(quoted, "\n"), strings.Join(written, "\n"))
	}
}
