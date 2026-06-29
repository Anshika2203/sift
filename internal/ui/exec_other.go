//go:build !windows

package ui

import (
	"os"
	"os/exec"
)

// runShell runs command via the system shell and returns its combined output.
func runShell(command string) string {
	out, _ := exec.Command("sh", "-c", command).CombinedOutput()
	return string(out)
}

// runInteractive runs command attached to the real terminal and returns its
// exit code (used by execute/become actions).
func runInteractive(command string) int {
	c := exec.Command("sh", "-c", command)
	c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := c.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return ee.ExitCode()
		}
		return 1
	}
	return 0
}
