//go:build windows

package ui

import (
	"os/exec"
	"syscall"
)

// runShell runs command via cmd.exe. We set the raw command line ourselves so
// that Go's argument escaping does not double-quote the command and cause cmd
// to mangle quoted arguments (e.g. `type "file"`).
func runShell(command string) string {
	c := exec.Command("cmd")
	c.SysProcAttr = &syscall.SysProcAttr{CmdLine: "cmd /c " + command}
	out, _ := c.CombinedOutput()
	return string(out)
}
