package cli

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/build"
	"github.com/mdlsvensson/moonwell/internal/diag"
)

func TestTheFirstInterruptCancelsTheCommandAndTheSecondLeaves(t *testing.T) {
	interrupts := make(chan os.Signal)
	cancelled, left := make(chan struct{}), make(chan struct{})
	go handleInterrupts(interrupts, func() { close(cancelled) }, func() { close(left) })
	interrupts <- os.Interrupt
	<-cancelled
	select {
	case <-left:
		t.Fatal("the first interrupt left the program")
	default:
	}
	interrupts <- os.Interrupt
	<-left
}

func TestLeavingAtOnceGivesBackTheBuildLockAndThenExitsWith130(t *testing.T) {
	root := t.TempDir()
	release, err := build.AcquireLock(root)
	if err != nil {
		t.Fatal(diag.Format(err))
	}
	defer release()
	lock := filepath.Join(root, "dist", ".lock")
	var codes []int
	leave := newForceExit(func(code int) {
		codes = append(codes, code)
		if _, err := os.Stat(lock); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("the program exits with the build lock in place: %v", err)
		}
	})
	if _, err := os.Stat(lock); err != nil || len(codes) != 0 {
		t.Fatalf("before the second interrupt: the lock: %v; exits: %v", err, codes)
	}
	leave()
	if !slices.Equal(codes, []int{130}) {
		t.Errorf("leaving exits with %v, want 130 once", codes)
	}
}
