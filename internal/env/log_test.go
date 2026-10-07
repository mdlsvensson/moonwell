package env

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"testing"
	"time"
)

func TestTheLoggerWritesToTheSinkAndAppendsToTheLogFile(t *testing.T) {
	file := filepath.Join(t.TempDir(), "nested", "moonwell.log")
	var lines []string
	logger := NewLogger(func(line string) { lines = append(lines, line) }, file)
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
	NewLogger(sink, "").Info("no file")
	// A folder in the log file's place: the line still reaches the sink.
	dir := t.TempDir()
	NewLogger(sink, dir).Info("a folder")
	// A file where the log file's folder should be.
	notAFolder := filepath.Join(dir, "dist")
	if err := os.WriteFile(notAFolder, nil, 0o666); err != nil {
		t.Fatal(err)
	}
	NewLogger(sink, filepath.Join(notAFolder, "moonwell.log")).Info("a file as the folder")
	if !slices.Equal(lines, []string{"no file", "a folder", "a file as the folder"}) {
		t.Errorf("the sink got %q", lines)
	}
}

func TestALogFileLineHasTheTimeInUTCWithMillisecondsAndTheLevel(t *testing.T) {
	twoHoursAhead := time.FixedZone("ahead", 2*60*60)
	at := time.Date(2026, time.October, 4, 1, 2, 3, 4_000_000, twoHoursAhead)
	if got, want := logLine(at, "warn", "warning: careful"), "[2026-10-03T23:02:03.004Z] warn: warning: careful\n"; got != want {
		t.Errorf("logLine = %q, want %q", got, want)
	}
}
