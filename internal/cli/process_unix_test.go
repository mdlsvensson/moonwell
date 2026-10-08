//go:build !windows

package cli

import (
	"os"
	"os/exec"
)

func apartFromTheTest(cmd *exec.Cmd) {}

func interruptDev(cmd *exec.Cmd) error { return cmd.Process.Signal(os.Interrupt) }

func sendInterrupt() {}
