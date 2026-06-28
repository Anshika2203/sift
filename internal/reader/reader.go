// Package reader gathers the list of candidate items sift will filter.
package reader

import (
	"bufio"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"golang.org/x/term"
)

// Read returns the items to search.
//
//   - If something is piped into standard input, each line becomes an item.
//   - Otherwise (stdin is an interactive terminal), sift runs the command in
//     $SIFT_DEFAULT_COMMAND if set, else walks the current directory listing
//     files (mirroring fzf's default behaviour).
//
// nul selects NUL ('\0') instead of newline as the line delimiter, for inputs
// produced by tools like `find -print0`.
func Read(nul bool) ([]string, error) {
	if term.IsTerminal(int(os.Stdin.Fd())) {
		if cmd := os.Getenv("SIFT_DEFAULT_COMMAND"); strings.TrimSpace(cmd) != "" {
			return readCommand(cmd, nul)
		}
		return walkFiles(".")
	}
	return readLines(os.Stdin, nul)
}

func readCommand(command string, nul bool) ([]string, error) {
	var c *exec.Cmd
	if runtime.GOOS == "windows" {
		c = exec.Command("cmd", "/c", command)
	} else {
		c = exec.Command("sh", "-c", command)
	}
	out, err := c.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := c.Start(); err != nil {
		return nil, err
	}
	items, _ := readLines(out, nul)
	_ = c.Wait()
	return items, nil
}

func readLines(r io.Reader, nul bool) ([]string, error) {
	delim := byte('\n')
	if nul {
		delim = 0
	}

	br := bufio.NewReaderSize(r, 64*1024)
	var items []string
	first := true
	for {
		line, err := br.ReadString(delim)
		if len(line) > 0 {
			line = strings.TrimRight(line, "\r\n\x00")
			if first {
				// Some producers (notably Windows PowerShell) prefix the stream
				// with a UTF-8 byte-order mark; drop it so it never becomes part
				// of an item's text.
				line = strings.TrimPrefix(line, "\ufeff")
				first = false
			}
			items = append(items, line)
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return items, err
		}
	}
	return items, nil
}

// walkFiles lists regular files under root, skipping hidden directories and
// common noise like .git so the default listing stays useful.
func walkFiles(root string) ([]string, error) {
	var items []string
	skipDirs := map[string]bool{".git": true, "node_modules": true, ".hg": true, ".svn": true}

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // ignore unreadable entries, keep walking
		}
		name := d.Name()
		if d.IsDir() {
			if path != root && (strings.HasPrefix(name, ".") || skipDirs[name]) {
				return filepath.SkipDir
			}
			return nil
		}
		// Present paths cleanly: strip a leading "./".
		items = append(items, strings.TrimPrefix(filepath.ToSlash(path), "./"))
		return nil
	})
	return items, err
}
