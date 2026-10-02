//go:build !windows

package cli

import (
	"os/exec"
	"syscall"
)

// SpawnDetached starts command without waiting for it; it keeps running after Moonwell exits. The game gets a
// session of its own, so a Ctrl+C in Moonwell's terminal does not reach it.
func SpawnDetached(command string, args []string) error {
	cmd := exec.Command(command, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}
