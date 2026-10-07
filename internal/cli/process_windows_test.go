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

// apartFromTheTest gives the program a hidden console and a process group of its own, so that an interrupt can
// be sent to it alone: not to the test runner, and not to the process that sends it.
func apartFromTheTest(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNewConsole | createNewProcessGroup}
}

// interruptDev sends the program an interrupt from a second copy of the test program, which joins the program's
// console to do so: a process can only signal the console it is attached to.
func interruptDev(cmd *exec.Cmd) error {
	sender := exec.Command(os.Args[0])
	sender.Env = append(os.Environ(), testRole+"="+interruptingRole, interruptedPid+"="+strconv.Itoa(cmd.Process.Pid))
	if output, err := sender.CombinedOutput(); err != nil {
		return fmt.Errorf("sending the interrupt: %w: %s", err, output)
	}
	return nil
}

// sendInterrupt is the sender. It sends Ctrl+Break, which Go reports as os.Interrupt just as Ctrl+C, to the
// program's process group. Ctrl+C would not do: it is switched off for a process started in a new process
// group and for everything below it, and it cannot be sent to one group. The sender must stay out of the
// event's way: joining a console removes the handlers Go installed, so the event would end it.
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
