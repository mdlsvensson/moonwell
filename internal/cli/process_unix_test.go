//go:build !windows

package cli

import (
	"os"
	"os/exec"
)

// apartFromTheTest has nothing to do where a signal is sent to one process.
func apartFromTheTest(cmd *exec.Cmd) {}

// interruptDev sends the program the signal that Ctrl+C sends.
func interruptDev(cmd *exec.Cmd) error { return cmd.Process.Signal(os.Interrupt) }

// sendInterrupt has nothing to do: the test sends the signal itself.
func sendInterrupt() {}
