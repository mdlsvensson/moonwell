package logging

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"testing"
)

func TestTheLoggerWritesToTheSinkAndAppendsToTheLogFile(t *testing.T) {
	file := filepath.Join(t.TempDir(), "nested", "moonwell.log")
	var lines []string
	logger := New(func(line string) { lines = append(lines, line) }, file)
	logger.Info("hello")
	logger.Warn("careful")
	logger.Error("error: broken")
	if !slices.Equal(lines, []string{"hello", "warning: careful", "error: broken"}) {
		t.Errorf("the sink got %q", lines)
	}
	content, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	want := regexp.MustCompile(`^\[\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d\.\d{3}Z\] info: hello\n` +
		`\[[^\]]+\] warn: warning: careful\n\[[^\]]+\] error: error: broken\n$`)
	if !want.Match(content) {
		t.Errorf("the log file holds %q", content)
	}
}

func TestALoggerWithoutAFileOrWithABrokenOneStillLogs(t *testing.T) {
	var lines []string
	sink := func(line string) { lines = append(lines, line) }
	New(sink, "").Info("no file")
	// A folder in the log file's place: the line still reaches the sink.
	dir := t.TempDir()
	New(sink, dir).Info("a folder")
	if !slices.Equal(lines, []string{"no file", "a folder"}) {
		t.Errorf("the sink got %q", lines)
	}
}
