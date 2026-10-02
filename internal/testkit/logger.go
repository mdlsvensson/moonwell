package testkit

import "github.com/mdlsvensson/moonwell/internal/logging"

// Recorder is a logger that records its lines instead of printing them.
type Recorder struct {
	*logging.Logger
	Lines []string
}

// NewRecorder returns a logger whose lines are kept in Lines.
func NewRecorder() *Recorder {
	recorder := &Recorder{}
	recorder.Logger = logging.New(func(line string) { recorder.Lines = append(recorder.Lines, line) }, "")
	return recorder
}
