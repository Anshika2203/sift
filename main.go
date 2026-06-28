// Command sift is a fast, interactive command-line fuzzy finder.
//
// It reads a list of items from standard input (or lists files in the current
// directory when stdin is a terminal), lets you narrow them down by typing a
// fuzzy query, and prints whatever you select.
package main

import (
	"bufio"
	_ "embed"
	"fmt"
	"os"
	"strings"

	"github.com/Anshika2203/sift/internal/matcher"
	"github.com/Anshika2203/sift/internal/reader"
	"github.com/Anshika2203/sift/internal/ui"
)

const version = "0.1.0"

//go:embed shell/key-bindings.bash
var bashBindings string

//go:embed shell/key-bindings.zsh
var zshBindings string

//go:embed shell/key-bindings.fish
var fishBindings string

const usage = `sift ` + version + ` - a command-line fuzzy finder

Usage:
  sift [options]
  <command> | sift [options]

Reads items from standard input (or lists files when stdin is a terminal),
lets you fuzzy-filter them interactively, and prints your selection.

Options:
  -q, --query STR     start with the given query
  -p, --prompt STR    set the input prompt (default "> ")
  -m, --multi         enable multi-select (Tab to mark, Enter to accept)
      --preview CMD   run CMD for the highlighted item; {} is the item
      --header STR    show a fixed header line above the list
  -f, --filter STR    non-interactive: print matches for STR and exit
      --read0         read NUL-separated input instead of newlines
      --bash          print bash key-binding script, then exit
      --zsh           print zsh key-binding script, then exit
      --fish          print fish key-binding script, then exit
  -V, --version       print version and exit
  -h, --help          show this help and exit

Keys:
  Up / Ctrl-P, Down / Ctrl-N   move cursor      PgUp / PgDn   page
  Enter                        accept           Esc / Ctrl-C  cancel
  Tab                          mark item (with --multi)
  Ctrl-U clear query     Ctrl-W delete word     Backspace delete char
`

type config struct {
	query     string
	prompt    string
	multi     bool
	preview   string
	header    string
	filter    string
	hasFilter bool
	read0     bool
}

// expandArgs splits "--key=value" into "--key" "value" so the parser only has
// to deal with space-separated values.
func expandArgs(args []string) []string {
	var out []string
	for _, a := range args {
		if strings.HasPrefix(a, "--") && strings.Contains(a, "=") {
			eq := strings.IndexByte(a, '=')
			out = append(out, a[:eq], a[eq+1:])
		} else {
			out = append(out, a)
		}
	}
	return out
}

func parseArgs(args []string) (config, error) {
	c := config{prompt: "> "}
	next := func(i *int, flag string) (string, error) {
		if *i+1 >= len(args) {
			return "", fmt.Errorf("missing value for %s", flag)
		}
		*i++
		return args[*i], nil
	}
	for i := 0; i < len(args); i++ {
		a := args[i]
		var err error
		switch a {
		case "-q", "--query":
			c.query, err = next(&i, a)
		case "-p", "--prompt":
			c.prompt, err = next(&i, a)
		case "-m", "--multi":
			c.multi = true
		case "--preview":
			c.preview, err = next(&i, a)
		case "--header":
			c.header, err = next(&i, a)
		case "-f", "--filter":
			c.filter, err = next(&i, a)
			c.hasFilter = true
		case "--read0":
			c.read0 = true
		case "-h", "--help":
			fmt.Print(usage)
			os.Exit(0)
		case "-V", "--version":
			fmt.Println("sift " + version)
			os.Exit(0)
		case "--bash":
			fmt.Println(strings.TrimSpace(bashBindings))
			os.Exit(0)
		case "--zsh":
			fmt.Println(strings.TrimSpace(zshBindings))
			os.Exit(0)
		case "--fish":
			fmt.Println(strings.TrimSpace(fishBindings))
			os.Exit(0)
		default:
			return c, fmt.Errorf("unknown option: %s (try --help)", a)
		}
		if err != nil {
			return c, err
		}
	}
	return c, nil
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "sift: "+err.Error())
	os.Exit(2)
}

func main() {
	cfg, err := parseArgs(expandArgs(os.Args[1:]))
	if err != nil {
		fail(err)
	}

	items, err := reader.Read(cfg.read0)
	if err != nil {
		fail(err)
	}

	// Non-interactive filter mode: print matches and exit.
	if cfg.hasFilter {
		matches := matcher.Filter(items, cfg.filter, false)
		w := bufio.NewWriter(os.Stdout)
		for _, mt := range matches {
			fmt.Fprintln(w, mt.Text)
		}
		w.Flush()
		if len(matches) == 0 {
			os.Exit(1)
		}
		return
	}

	res, err := ui.Run(items, ui.Options{
		Prompt:  cfg.prompt,
		Query:   cfg.query,
		Multi:   cfg.multi,
		Preview: cfg.preview,
		Header:  cfg.header,
	})
	if err != nil {
		fail(err)
	}
	if res.Aborted {
		os.Exit(130)
	}

	w := bufio.NewWriter(os.Stdout)
	for _, s := range res.Selected {
		fmt.Fprintln(w, s)
	}
	w.Flush()
}
