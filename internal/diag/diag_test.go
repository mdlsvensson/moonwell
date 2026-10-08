package diag

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestFormatRendersAnExpectedFailure(t *testing.T) {
	for _, c := range []struct {
		name string
		err  error
		want string
	}{
		{
			"the file, the line, the message and the hint",
			&Error{Msg: "unexpected symbol", File: "src/main.yue", Line: 3, Hint: "check the indentation"},
			"error: src/main.yue:3 › unexpected symbol\nhint: check the indentation",
		},
		{"no location and no hint", &Error{Msg: "no project"}, "error: no project"},
		{"a file without a line", &Error{Msg: "bad", File: "moonwell.pkl"}, "error: moonwell.pkl › bad"},
		{
			"an error wrapped in another",
			fmt.Errorf("while building: %w", &Error{Msg: "bad", File: "moonwell.pkl"}),
			"error: moonwell.pkl › bad",
		},
		{
			"each problem on its own line with its own hint",
			Problems{
				{File: "objects/a.pkl", Msg: `units["a"].id: bad`, Hint: "fix it"},
				{File: "objects/b.pkl", Msg: `units["b"].base: worse`},
			},
			"error: objects/a.pkl › units[\"a\"].id: bad\nhint: fix it\nerror: objects/b.pkl › units[\"b\"].base: worse",
		},
		{
			"each problem with its line and column",
			Problems{
				{
					File: "src/main.yue", Line: 7, Column: 11,
					Msg: "Unknown global CreatUnit.", Hint: "Did you mean CreateUnit?",
				},
				{File: "src/main.yue", Line: 9, Msg: "second"},
				{File: "objects/a.pkl", Msg: "third"},
			},
			strings.Join([]string{
				"error: src/main.yue:7:11 › Unknown global CreatUnit.",
				"hint: Did you mean CreateUnit?",
				"error: src/main.yue:9 › second",
				"error: objects/a.pkl › third",
			}, "\n"),
		},
		{
			"problems wrapped in another error",
			fmt.Errorf("while checking: %w", Problems{{File: "objects/a.pkl", Msg: "first"}}),
			"error: objects/a.pkl › first",
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := Format(c.err); got != c.want {
				t.Errorf("Format = %q, want %q", got, c.want)
			}
		})
	}
}

func TestFormatTreatsAnyOtherErrorAsInternal(t *testing.T) {
	got := Format(errors.New("boom"))
	for _, want := range []string{"internal error:", "boom", "please report"} {
		if !strings.Contains(got, want) {
			t.Errorf("Format = %q, want it to contain %q", got, want)
		}
	}
	if got != FormatInternalError("boom") {
		t.Errorf("Format = %q, want what Internal renders: %q", got, FormatInternalError("boom"))
	}
}

func TestFormatPrintsAtMostMaxProblemsThenHowManyMore(t *testing.T) {
	var many Problems
	for i := range MaxProblems + 5 {
		many = append(many, Problem{File: "f.pkl", Msg: fmt.Sprintf("p%d", i)})
	}
	lines := strings.Split(Format(many), "\n")
	if len(lines) != MaxProblems+1 || lines[0] != "error: f.pkl › p0" || lines[19] != "error: f.pkl › p19" ||
		lines[20] != "and 5 more" {
		t.Errorf("Format of %d problems = %q", len(many), lines)
	}
	if got := len(strings.Split(Format(many[:MaxProblems]), "\n")); got != MaxProblems {
		t.Errorf("%d problems print %d lines", MaxProblems, got)
	}
}

func TestFormatProblemHasNoPrefixSoAWarningCanUseIt(t *testing.T) {
	for _, c := range []struct {
		name    string
		problem Problem
		want    string
	}{
		{
			"everything",
			Problem{File: "src/a.yue", Line: 1, Column: 2, Msg: "m", Hint: "h"},
			"src/a.yue:1:2 › m\nhint: h",
		},
		{"the message alone", Problem{Msg: "m"}, "m"},
		{"a column is shown only with its line", Problem{File: "src/a.yue", Column: 2, Msg: "m"}, "src/a.yue › m"},
		{"a line is shown only with its file", Problem{Line: 1, Column: 2, Msg: "m"}, "m"},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := FormatProblem(c.problem); got != c.want {
				t.Errorf("FormatProblem = %q, want %q", got, c.want)
			}
		})
	}
}

func TestFirstIsAnErrorAsOneProblem(t *testing.T) {
	problems := Problems{
		{File: "src/main.yue", Line: 7, Column: 11, Msg: "first", Hint: "one"},
		{File: "objects/b.pkl", Msg: "second"},
	}
	failure := &Error{Msg: "bad", File: "moonwell.pkl", Line: 4, Column: 2, Hint: "mend it", Cause: errors.New("x")}
	for _, c := range []struct {
		name string
		err  error
		want Problem
		ok   bool
	}{
		{"the first of several problems", problems, problems[0], true},
		{
			"the fields of an Error",
			failure,
			Problem{File: "moonwell.pkl", Line: 4, Column: 2, Msg: "bad", Hint: "mend it"},
			true,
		},
		{
			"an Error wrapped in another error",
			fmt.Errorf("while building: %w", failure),
			Problem{File: "moonwell.pkl", Line: 4, Column: 2, Msg: "bad", Hint: "mend it"},
			true,
		},
		{"an error that is not Moonwell's", errors.New("other"), Problem{}, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got, ok := FirstProblem(c.err); got != c.want || ok != c.ok {
				t.Errorf("First = %+v, %v; want %+v, %v", got, ok, c.want, c.ok)
			}
		})
	}
}

func TestAnErrorReadsAsItsMessageAndUnwrapsToItsCause(t *testing.T) {
	cause := errors.New("access denied")
	failure := &Error{Msg: "cannot write the map", File: "dist/map.w3x", Cause: cause}
	if failure.Error() != "cannot write the map" {
		t.Errorf("Error() = %q", failure.Error())
	}
	if !errors.Is(failure, cause) {
		t.Error("errors.Is does not find the cause of an Error")
	}
	problems := Problems{{File: "objects/a.pkl", Msg: "first"}, {File: "objects/b.pkl", Msg: "second"}}
	if problems.Error() != "first" {
		t.Errorf("Problems.Error() = %q, want the first problem's message", problems.Error())
	}
}
