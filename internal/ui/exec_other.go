//go:build !windows

package ui

import "os/exec"

// runShell runs command via the system shell and returns its combined output.
func runShell(command string) string {
	out, _ := exec.Command("sh", "-c", command).CombinedOutput()
	return string(out)
}
