//go:build windows

package ui

import (
	"os"
	"os/exec"
	"syscall"
)

// runShell runs command via cmd.exe. The raw command line is set explicitly so
// Go's argument escaping does not double-quote it (which made cmd mangle quoted
// arguments such as `type "file"`).
func runShell(command string) string {
	c := exec.Command("cmd")
	c.SysProcAttr = &syscall.SysProcAttr{CmdLine: "cmd /c " + command}
	out, _ := c.CombinedOutput()
	return string(out)
}

// runInteractive runs command attached to the real terminal and returns its
// exit code (used by execute/become actions).
func runInteractive(command string) int {
	c := exec.Command("cmd")
	c.SysProcAttr = &syscall.SysProcAttr{CmdLine: "cmd /c " + command}
	c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := c.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return ee.ExitCode()
		}
		return 1
	}
	return 0
}
