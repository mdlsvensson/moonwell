package cli_test

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"syscall"
)

const (
	createNewConsole = 0x10
	ctrlBreakEvent   = 1
)

// configureDevProcess gives the program a hidden console of its own, so that an interrupt sent to that console
// reaches the program and not the test runner.
func configureDevProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNewConsole}
}

// interruptDevProcess sends the program an interrupt from a second copy of the test program, which joins the
// program's console to do so: a process can only signal the console it is attached to.
func interruptDevProcess(cmd *exec.Cmd) error {
	sender := exec.Command(os.Args[0])
	sender.Env = append(os.Environ(), "MOONWELL_TEST_ROLE=interrupt", "MOONWELL_TEST_INTERRUPT_PID="+strconv.Itoa(cmd.Process.Pid))
	if output, err := sender.CombinedOutput(); err != nil {
		return fmt.Errorf("sending the interrupt: %w: %s", err, output)
	}
	return nil
}

// processInterruptHelper is the sender. It sends Ctrl+Break, which Go reports as os.Interrupt just like Ctrl+C:
// Ctrl+C is switched off for every process below one started in a new process group, as test runners often are, and
// Ctrl+Break cannot be switched off.
func processInterruptHelper() {
	kernel := syscall.NewLazyDLL("kernel32.dll")
	pid, err := strconv.Atoi(os.Getenv("MOONWELL_TEST_INTERRUPT_PID"))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	// The event goes to every process on the console, this one included: taking the signal keeps this one alive.
	// (signal.Ignore does not: Go then leaves the event to Windows, which ends the process.)
	signal.Notify(make(chan os.Signal, 1), os.Interrupt)
	kernel.NewProc("FreeConsole").Call()
	if result, _, err := kernel.NewProc("AttachConsole").Call(uintptr(pid)); result == 0 {
		fmt.Fprintln(os.Stderr, "AttachConsole:", err)
		os.Exit(1)
	}
	if result, _, err := kernel.NewProc("GenerateConsoleCtrlEvent").Call(ctrlBreakEvent, 0); result == 0 {
		fmt.Fprintln(os.Stderr, "GenerateConsoleCtrlEvent:", err)
		os.Exit(1)
	}
	kernel.NewProc("FreeConsole").Call()
}
