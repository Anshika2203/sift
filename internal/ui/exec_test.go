package ui

import (
	"os"
	"runtime"
	"strings"
	"testing"
)

// TestRunShellQuotedArg guards against the Windows quoting bug where a preview
// command with a quoted path (e.g. `type "file"`) was mangled by cmd.exe.
func TestRunShellQuotedArg(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "siftrun*.txt")
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString("PREVIEW-OK")
	f.Close()

	cmd := "cat " + shellQuote(f.Name())
	if runtime.GOOS == "windows" {
		cmd = "type " + shellQuote(f.Name())
	}
	out := runShell(cmd)
	if !strings.Contains(out, "PREVIEW-OK") {
		t.Fatalf("runShell(%q) = %q, want it to contain PREVIEW-OK", cmd, out)
	}
}
