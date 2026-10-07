package env

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Logger passes every line to a sink (the terminal) and appends it to a log file with the time and the level.
// Logging never fails: a log file that cannot be written must not fail a command.
type Logger struct {
	write func(line string)
	file  string
}

// NewLogger returns a logger that passes each line to write and appends it to file. An empty file keeps no log.
func NewLogger(write func(line string), file string) *Logger {
	return &Logger{write: write, file: file}
}

// StderrLogger returns a logger that prints to stderr and appends to file.
func StderrLogger(file string) *Logger {
	return NewLogger(func(line string) { fmt.Fprintln(os.Stderr, line) }, file)
}

// Info logs a line.
func (l *Logger) Info(message string) { l.log("info", message) }

// Warn logs a line with the prefix "warning: ".
func (l *Logger) Warn(message string) { l.log("warn", "warning: "+message) }

// Error logs a line as it is; the caller formats it.
func (l *Logger) Error(message string) { l.log("error", message) }

// log sends message to the sink and then to the log file. The sink comes first, so the user reads the line even
// when the file fails.
func (l *Logger) log(level, message string) {
	l.write(message)
	if l.file != "" {
		appendLine(l.file, logLine(time.Now(), level, message))
	}
}

// appendLine adds line to the end of file, creating the file and its folder. A failure is dropped: there is nowhere
// better to report it than the sink, which already has the line.
func appendLine(file, line string) {
	if err := os.MkdirAll(filepath.Dir(file), 0o777); err != nil {
		return
	}
	out, err := os.OpenFile(file, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o666)
	if err != nil {
		return
	}
	defer out.Close()
	out.WriteString(line)
}

// logLine is a line of the log file: `[time] level: message`, with the time in UTC to the millisecond, so that
// lines written in different time zones sort and compare.
func logLine(at time.Time, level, message string) string {
	return fmt.Sprintf("[%s] %s: %s\n", at.UTC().Format("2006-01-02T15:04:05.000Z"), level, message)
}
