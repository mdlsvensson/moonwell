//go:build !windows

package cli_test

import (
	"os"
	"os/exec"
)

func configureDevProcess(cmd *exec.Cmd)       {}
func interruptDevProcess(cmd *exec.Cmd) error { return cmd.Process.Signal(os.Interrupt) }
func processInterruptHelper()                 {}
