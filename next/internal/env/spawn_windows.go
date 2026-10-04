package env

import (
	"os/exec"
	"syscall"
)

// Process creation flags of Windows.
const (
	detachedProcess       = 0x00000008
	createNewProcessGroup = 0x00000200
)

// SpawnDetached starts program without waiting for it; it keeps running after Moonwell exits. The program gets no
// console and a process group of its own, so a Ctrl+C in Moonwell's terminal does not reach it.
func SpawnDetached(program string, args []string) error {
	cmd := exec.Command(program, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: detachedProcess | createNewProcessGroup}
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}
