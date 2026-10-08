//go:build !windows

package env

import (
	"os/exec"
	"syscall"
)

func SpawnDetached(program string, args []string) error {
	cmd := exec.Command(program, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}
