//go:build !windows

package testkit

import (
	"os"
	"testing"
)

func MakeUnreadable(t testing.TB, path string) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("root may read a file without permissions, so the read would not fail")
	}
	if err := os.Chmod(path, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(path, 0o666) })
}

func MakeUnwritable(t testing.TB, path string) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("root may write a file without the permission, so the write would not fail")
	}
	if err := os.Chmod(path, 0o444); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(path, 0o666) })
}
