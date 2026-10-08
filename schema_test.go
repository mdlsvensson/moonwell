package moonwell

import (
	"os"
	"os/exec"
	"testing"
)

func TestThePklSchemasOwnTestsPass(t *testing.T) {
	pkl, err := exec.LookPath("pkl")
	if err != nil {
		if os.Getenv("MOONWELL_REQUIRE_TOOLS") == "1" {
			t.Fatal("pkl is not on the PATH, and MOONWELL_REQUIRE_TOOLS=1 requires it")
		}
		t.Skip("pkl is not on the PATH")
	}
	output, err := exec.Command(pkl, "test", "schema/tests/Project.pkl", "schema/tests/Objects.pkl").CombinedOutput()
	if err != nil {
		t.Errorf("pkl test failed: %v\n%s", err, output)
	}
}
