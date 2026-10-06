package testkit

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/mdlsvensson/moonwell/next/internal/diag"
)

// Refusal is what a package says of one input it refuses, as a recording of refusals holds it: the name of the
// input, and the file, the message, the hint and the place of the error. An error that is no *diag.Error has
// its text for a message and nothing else.
type Refusal struct {
	Input         string
	File, Message string
	Hint          string
	Line, Column  int
}

// RefusalOf is what err says of the input it refuses. An input that is not refused, err being nil, has a
// message that says so, which a recording then shows.
func RefusalOf(input string, err error) Refusal {
	var failure *diag.Error
	switch {
	case err == nil:
		return Refusal{Input: input, Message: "(not refused)"}
	case errors.As(err, &failure):
		return Refusal{input, failure.File, failure.Msg, failure.Hint, failure.Line, failure.Column}
	}
	return Refusal{Input: input, Message: err.Error()}
}

// Refusals is the text of a recording of refusals. Each message stands once, with its file and its hint below
// it where it has them, and below those the inputs that are refused with it, each with its place where it has
// one. The messages are in the order of their first input, and the inputs of a message in the order given. So
// a diff of two recordings shows a word of a message that changed once, and an input that is refused for
// another reason as a line that moved. A name or a message with a character that is not printed is quoted as
// Go quotes a string.
func Refusals(refusals []Refusal) []byte {
	type words struct{ message, file, hint string }
	var order []words
	inputs := map[words][]string{}
	for _, refusal := range refusals {
		key := words{refusal.Message, refusal.File, refusal.Hint}
		if _, seen := inputs[key]; !seen {
			order = append(order, key)
		}
		input := printed(refusal.Input)
		if refusal.Line != 0 || refusal.Column != 0 {
			input += fmt.Sprintf(" (at %d:%d)", refusal.Line, refusal.Column)
		}
		inputs[key] = append(inputs[key], input)
	}
	var text bytes.Buffer
	for _, key := range order {
		fmt.Fprintf(&text, "%s\n", printed(key.message))
		if key.file != "" {
			fmt.Fprintf(&text, "    file: %s\n", printed(key.file))
		}
		if key.hint != "" {
			fmt.Fprintf(&text, "    hint: %s\n", printed(key.hint))
		}
		for _, input := range inputs[key] {
			fmt.Fprintf(&text, "    - %s\n", input)
		}
		text.WriteString("\n")
	}
	return text.Bytes()
}

// printed is text as a line of a recording holds it: as it is when every character of it is printed, and
// quoted as Go quotes a string when one is not.
func printed(text string) string {
	if strings.ContainsFunc(text, func(r rune) bool { return !strconv.IsPrint(r) }) {
		return strconv.Quote(text)
	}
	return text
}
