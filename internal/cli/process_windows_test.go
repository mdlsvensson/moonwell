package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"syscall"
)

const (
	createNewConsole      = 0x10
	createNewProcessGroup = 0x200
	ctrlBreakEvent        = 1
)

func detachFromTest(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNewConsole | createNewProcessGroup}
}

func interruptDev(cmd *exec.Cmd) error {
	sender := exec.Command(os.Args[0])
	sender.Env = append(os.Environ(), testRole+"="+interruptingRole, interruptedPid+"="+strconv.Itoa(cmd.Process.Pid))
	if output, err := sender.CombinedOutput(); err != nil {
		return fmt.Errorf("sending the interrupt: %w: %s", err, output)
	}
	return nil
}

func sendInterrupt() {
	kernel := syscall.NewLazyDLL("kernel32.dll")
	pid, err := strconv.Atoi(os.Getenv(interruptedPid))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	kernel.NewProc("FreeConsole").Call()
	if result, _, err := kernel.NewProc("AttachConsole").Call(uintptr(pid)); result == 0 {
		fmt.Fprintln(os.Stderr, "AttachConsole:", err)
		os.Exit(1)
	}
	if result, _, err := kernel.NewProc("GenerateConsoleCtrlEvent").Call(ctrlBreakEvent, uintptr(pid)); result == 0 {
		fmt.Fprintln(os.Stderr, "GenerateConsoleCtrlEvent:", err)
		os.Exit(1)
	}
	kernel.NewProc("FreeConsole").Call()
}
