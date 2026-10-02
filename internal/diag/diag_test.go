package diag

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestFormatPrintsFileLineMessageAndHint(t *testing.T) {
	err := &Error{Msg: "unexpected symbol", File: "src/main.yue", Line: 3, Hint: "check the indentation"}
	want := "error: src/main.yue:3 › unexpected symbol\nhint: check the indentation"
	if got := Format(err); got != want {
		t.Errorf("Format = %q, want %q", got, want)
	}
}

func TestFormatWithoutLocationOrHint(t *testing.T) {
	if got := Format(&Error{Msg: "no project"}); got != "error: no project" {
		t.Errorf("Format = %q", got)
	}
}

func TestFormatWithFileButNoLine(t *testing.T) {
	if got := Format(&Error{Msg: "bad", File: "moonwell.pkl"}); got != "error: moonwell.pkl › bad" {
		t.Errorf("Format = %q", got)
	}
}

func TestFormatTreatsOtherErrorsAsInternal(t *testing.T) {
	got := Format(errors.New("boom"))
	for _, want := range []string{"internal error:", "boom", "please report"} {
		if !strings.Contains(got, want) {
			t.Errorf("Format = %q, want it to contain %q", got, want)
		}
	}
}

func TestFormatSeesAnErrorThroughWrapping(t *testing.T) {
	wrapped := fmt.Errorf("while building: %w", &Error{Msg: "bad", File: "moonwell.pkl"})
	if got := Format(wrapped); got != "error: moonwell.pkl › bad" {
		t.Errorf("Format = %q", got)
	}
}

func TestFirstIsTheFirstProblem(t *testing.T) {
	problems := Problems{
		{File: "objects/a.pkl", Msg: "first", Hint: "one"},
		{File: "objects/b.pkl", Msg: "second"},
	}
	first, ok := First(problems)
	if !ok || first.File != "objects/a.pkl" || first.Msg != "first" || first.Hint != "one" {
		t.Errorf("First = %+v, %v", first, ok)
	}
	if problems.Error() != "first" {
		t.Errorf("Error() = %q", problems.Error())
	}
	if _, ok := First(errors.New("other")); ok {
		t.Error("First accepted an error that is not Moonwell's")
	}
}

func TestFormatPrintsEachProblemWithItsHintAtMostTwentyThenHowManyMore(t *testing.T) {
	got := Format(Problems{
		{File: "objects/a.pkl", Msg: `units["a"].id: bad`, Hint: "fix it"},
		{File: "objects/b.pkl", Msg: `units["b"].base: worse`},
	})
	want := "error: objects/a.pkl › units[\"a\"].id: bad\nhint: fix it\nerror: objects/b.pkl › units[\"b\"].base: worse"
	if got != want {
		t.Errorf("Format = %q, want %q", got, want)
	}

	var many Problems
	for i := range 25 {
		many = append(many, Problem{File: "f.pkl", Msg: fmt.Sprintf("p%d", i)})
	}
	lines := strings.Split(Format(many), "\n")
	if len(lines) != 21 || lines[0] != "error: f.pkl › p0" || lines[19] != "error: f.pkl › p19" ||
		lines[20] != "and 5 more" {
		t.Errorf("Format of 25 problems = %q", lines)
	}
	if got := len(strings.Split(Format(many[:20]), "\n")); got != 20 {
		t.Errorf("20 problems print %d lines", got)
	}
}

func TestFormatPrintsEachProblemWithItsLineAndColumn(t *testing.T) {
	problems := Problems{
		{File: "src/main.yue", Line: 7, Column: 11, Msg: "Unknown global CreatUnit.", Hint: "Did you mean CreateUnit?"},
		{File: "src/main.yue", Line: 9, Msg: "second"},
		{File: "objects/a.pkl", Msg: "third"},
	}
	want := strings.Join([]string{
		"error: src/main.yue:7:11 › Unknown global CreatUnit.",
		"hint: Did you mean CreateUnit?",
		"error: src/main.yue:9 › second",
		"error: objects/a.pkl › third",
	}, "\n")
	if got := Format(problems); got != want {
		t.Errorf("Format = %q, want %q", got, want)
	}
	first, _ := First(problems)
	if first.File != "src/main.yue" || first.Line != 7 || first.Msg != "Unknown global CreatUnit." {
		t.Errorf("First = %+v", first)
	}
}

func TestFormatProblemHasNoPrefixSoWarningsCanUseIt(t *testing.T) {
	got := FormatProblem(Problem{File: "src/a.yue", Line: 1, Column: 2, Msg: "m", Hint: "h"})
	if got != "src/a.yue:1:2 › m\nhint: h" {
		t.Errorf("FormatProblem = %q", got)
	}
	if got := FormatProblem(Problem{Msg: "m"}); got != "m" {
		t.Errorf("FormatProblem = %q", got)
	}
}
