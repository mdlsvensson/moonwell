// Command moonwell is the Moonwell command line: a toolchain for Warcraft III maps with YueScript gameplay and Pkl
// data.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/mdlsvensson/moonwell/internal/cli"
	"github.com/mdlsvensson/moonwell/internal/pipeline"
)

func main() {
	root, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error: Cannot tell which folder this is: "+err.Error())
		os.Exit(1)
	}
	// The first Ctrl+C asks the command to stop: dev stops watching once its check has finished, assets:sync undoes
	// what it wrote, and the others stop at their next waiting point. The second one leaves at once, without a stale
	// build lock.
	ctx, cancel := context.WithCancel(context.Background())
	interrupts := make(chan os.Signal, 2)
	signal.Notify(interrupts, os.Interrupt)
	go func() {
		<-interrupts
		cancel()
		<-interrupts
		pipeline.ReleaseHeldLocks()
		os.Exit(130)
	}()
	write := func(line string) { fmt.Fprintln(os.Stderr, line) }
	print := func(text string) { fmt.Fprintln(os.Stdout, text) }
	os.Exit(cli.Run(ctx, os.Args[1:], root, write, print))
}
