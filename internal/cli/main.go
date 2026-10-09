package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/mdlsvensson/moonwell/internal/build"
	"github.com/mdlsvensson/moonwell/internal/diag"
)

func Main() int {
	writeStderr := func(line string) { fmt.Fprintln(os.Stderr, line) }
	writeStdout := func(text string) { fmt.Fprintln(os.Stdout, text) }
	workDir, err := os.Getwd()
	if err != nil {
		writeStderr(diag.Format(errNoWorkingFolder(err)))
		return 1
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	interrupts := make(chan os.Signal, 2)
	signal.Notify(interrupts, os.Interrupt)
	go handleInterrupts(interrupts, cancel, newForceExit(os.Exit))
	return Run(ctx, os.Args[1:], workDir, writeStderr, writeStdout)
}

func handleInterrupts(interrupts <-chan os.Signal, cancel, forceExit func()) {
	<-interrupts
	cancel()
	<-interrupts
	forceExit()
}

func newForceExit(exit func(int)) func() {
	return func() {
		build.ReleaseHeldLocks()
		exit(130)
	}
}

func errNoWorkingFolder(cause error) error {
	return &diag.Error{
		Msg:   "Cannot tell which folder this is: " + cause.Error(),
		Hint:  "Run moonwell from a folder that is there and that you may read.",
		Cause: cause,
	}
}
