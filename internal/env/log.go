package env

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type Logger struct {
	write func(line string)
	file  string
}

func NewLogger(write func(line string), file string) *Logger {
	return &Logger{write: write, file: file}
}

func StderrLogger(file string) *Logger {
	return NewLogger(func(line string) { fmt.Fprintln(os.Stderr, line) }, file)
}

func (l *Logger) Info(message string) { l.log("info", message) }

func (l *Logger) Warn(message string) { l.log("warn", "warning: "+message) }

func (l *Logger) Error(message string) { l.log("error", message) }

func (l *Logger) log(level, message string) {
	l.write(message)
	if l.file != "" {
		appendLine(l.file, logLine(time.Now(), level, message))
	}
}

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

func logLine(at time.Time, level, message string) string {
	return fmt.Sprintf("[%s] %s: %s\n", at.UTC().Format("2006-01-02T15:04:05.000Z"), level, message)
}
