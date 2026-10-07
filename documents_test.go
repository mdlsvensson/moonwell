package moonwell

import (
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// The documents for contributors name files and folders of the repository, and a name that is not there sends a
// reader nowhere. So every path they name must be there.
//
// A path is what stands between two backticks on a line outside a block of code, when it has no space and none
// of < > * ( and the ellipsis, and when it either starts with one of the repository's folders (folders, below) or
// has no "/" and ends as a file at the root does: .go, .md, .mod, .ps1 or .sh. So a document names a file by its
// whole path from the root, internal/build/build.go: build.go alone is looked for at the root, and fails.
// Anything else between backticks is not looked at: a name of the code, a command line, a pattern such as
// schema/generated/*.pkl, and a file of a user's project, such as dist/stage or moonwell.pkl.
func TestTheDocumentsNameFilesAndFoldersThatAreThere(t *testing.T) {
	folders := []string{".github", "cmd", "data", "internal", "runtime", "schema", "template", "tools"}
	const ellipsis = "\xE2\x80\xA6" // U+2026, which a document writes for "and so on": internal/war3/...
	atTheRoot := regexp.MustCompile(`^[^/]+\.(go|md|mod|ps1|sh)$`)
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
				named := slices.Contains(folders, first) || atTheRoot.MatchString(path)
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
