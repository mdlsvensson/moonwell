package cli

import (
	"os/exec"
	"syscall"
)

const (
	detachedProcess       = 0x00000008
	createNewProcessGroup = 0x00000200
)

// SpawnDetached starts command without waiting for it; it keeps running after Moonwell exits. The game gets no
// console and a process group of its own, so a Ctrl+C in Moonwell's terminal does not reach it.
func SpawnDetached(command string, args []string) error {
	cmd := exec.Command(command, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: detachedProcess | createNewProcessGroup}
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}
