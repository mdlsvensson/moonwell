//go:build !windows

package env

import (
	"os/exec"
	"syscall"
)

// SpawnDetached starts program without waiting for it; it keeps running after Moonwell exits. The program gets a
// session of its own, so a Ctrl+C in Moonwell's terminal does not reach it.
func SpawnDetached(program string, args []string) error {
	cmd := exec.Command(program, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}
