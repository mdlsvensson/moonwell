package manifest

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	moonwell "github.com/mdlsvensson/moonwell"
	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/env"
	"github.com/mdlsvensson/moonwell/internal/testkit"
)

var background = context.Background()

func ptr[T any](value T) *T { return &value }

func derefOrNil(value *string) string {
	if value == nil {
		return "<nil>"
	}
	return *value
}

func asDiagError(t *testing.T, err error, what string) *diag.Error {
	t.Helper()
	var diagErr *diag.Error
	if !errors.As(err, &diagErr) {
		t.Fatalf("%s: got %v, want a *diag.Error", what, err)
	}
	return diagErr
}

func newEnv(t *testing.T, files map[string]string) *env.Env {
	t.Helper()
	root := t.TempDir()
	for name, content := range files {
		testkit.WriteFile(t, root, name, []byte(content))
	}
	e, _ := testkit.Env(t, root)
	return e
}

type runCall struct{ line, dir string }

func fakeRun(calls *[]runCall, results map[string]env.RunResult) env.RunFunc {
	return func(_ context.Context, program string, args []string, options env.RunOptions) (env.RunResult, error) {
		line := strings.Join(append([]string{program}, args...), " ")
		*calls = append(*calls, runCall{line, options.Dir})
		best, known := "", false
		for prefix := range results {
			if strings.HasPrefix(line, prefix) && len(prefix) >= len(best) {
				best, known = prefix, true
			}
		}
		if !known {
			return env.RunResult{}, errors.New("unexpected command: " + line)
		}
		return results[best], nil
	}
}

func newLinkedEnv(t *testing.T, files map[string]string) (*env.Env, string) {
	t.Helper()
	pkl := testkit.NeedPkl(t)
	e := newEnv(t, files)
	e.Run = env.Run
	schema, err := filepath.Rel(e.Root, filepath.Join(testkit.RepoRoot(t), "schema"))
	if err != nil {
		t.Fatalf("the temporary folder %s must be on the drive of the checkout: %v", e.Root, err)
	}
	testkit.WriteFile(t, e.Root, "PklProject", []byte(PklProjectText(moonwell.Version, filepath.ToSlash(schema))))
	resolved, err := e.Run(background, pkl, []string{"project", "resolve"}, env.RunOptions{Dir: e.Root})
	if err != nil || resolved.ExitCode != 0 {
		t.Fatalf("pkl project resolve: %v\n%s", err, resolved.Stderr)
	}
	return e, pkl
}
