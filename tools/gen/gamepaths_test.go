package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateGamePathsWritesTheListAndReturnsThePathCount(t *testing.T) {
	target := filepath.Join(t.TempDir(), "game-paths.txt")
	count, err := GenerateGamePaths("war3.w3mod:Units/Human/Footman/Footman.mdx\n", "2.0.0", target)
	if err != nil || count != 1 {
		t.Fatalf("count %d, error %v", count, err)
	}
	if written, _ := os.ReadFile(target); string(written) != "# Warcraft III 2.0.0\nunits/human/footman/footman.mdx\n" {
		t.Errorf("wrote %q", written)
	}
}

func TestGenerateGamePathsFailsAndKeepsTheExistingListWhenNoPathIsRecognized(t *testing.T) {
	target := filepath.Join(t.TempDir(), "game-paths.txt")
	existing := "# Warcraft III 1.0.0\nunits/old.mdx\n"
	if err := os.WriteFile(target, []byte(existing), 0o666); err != nil {
		t.Fatal(err)
	}
	if _, err := GenerateGamePaths("war3.w3mod:Sound/Hit.wav\n", "2.0.0", target); err == nil {
		t.Error("a listfile without a model or texture path was accepted")
	}
	if written, _ := os.ReadFile(target); string(written) != existing {
		t.Errorf("the list changed to %q", written)
	}
}
