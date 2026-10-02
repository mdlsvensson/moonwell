// Package logging is Moonwell's logger: every line goes to a sink (the terminal's stderr) and, inside a project,
// to dist/moonwell.log with a timestamp.
package logging

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Logger writes lines. Logging never fails: a broken log file must never fail a build.
type Logger struct {
	write func(line string)
	file  string
}

// New returns a logger that passes each line to write and appends it to file. An empty file keeps no log.
func New(write func(line string), file string) *Logger {
	return &Logger{write: write, file: file}
}

// Stderr returns a logger that prints to stderr and appends to file.
func Stderr(file string) *Logger {
	return New(func(line string) { fmt.Fprintln(os.Stderr, line) }, file)
}

func (l *Logger) log(level, message string) {
	l.write(message)
	if l.file == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(l.file), 0o777); err != nil {
		return
	}
	file, err := os.OpenFile(l.file, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o666)
	if err != nil {
		return
	}
	defer file.Close()
	// As JavaScript's toISOString: UTC with milliseconds.
	fmt.Fprintf(file, "[%s] %s: %s\n", time.Now().UTC().Format("2006-01-02T15:04:05.000Z"), level, message)
}

// Info logs a line.
func (l *Logger) Info(message string) { l.log("info", message) }

// Warn logs a line with the prefix "warning: ".
func (l *Logger) Warn(message string) { l.log("warn", "warning: "+message) }

// Error logs a line as it is; the caller formats it.
func (l *Logger) Error(message string) { l.log("error", message) }
