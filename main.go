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
	"github.com/Anshika2203/sift/internal/pattern"
	"github.com/Anshika2203/sift/internal/reader"
	"github.com/Anshika2203/sift/internal/ui"
)

const version = "0.2.0"

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

Reads items from standard input (or runs $SIFT_DEFAULT_COMMAND / lists files
when stdin is a terminal), lets you fuzzy-filter them interactively, and prints
your selection.

Search:
  -e, --exact            exact-match by default (' prefix flips to fuzzy)
  -i, --ignore-case      case-insensitive matching
  +i, --no-ignore-case   case-sensitive matching
      --literal          accepted for compatibility (no normalization applied)
  +s, --no-sort          do not sort the result by score
      --tac              reverse the order of the input

Interface:
  -q, --query STR        start with the given query
  -p, --prompt STR       set the input prompt (default "> ")
  -m, --multi            enable multi-select (Tab to mark, Enter to accept)
      --preview CMD      run CMD for the highlighted item; {} is the item
      --header STR       fixed header line(s) shown above the list
      --header-lines N   treat the first N input lines as a sticky header

Scripting:
  -f, --filter STR       non-interactive: print matches for STR and exit
  -1, --select-1         if only one item matches, select it without the UI
  -0, --exit-0           if no item matches, exit immediately
      --print-query      print the final query as the first output line
      --expect KEYS      comma-separated keys that accept and report themselves
      --read0            read NUL-separated input
      --print0           print NUL-separated output

Shell integration:
      --bash             print bash key-binding script, then exit
      --zsh              print zsh key-binding script, then exit
      --fish             print fish key-binding script, then exit

  -V, --version          print version and exit
  -h, --help             show this help and exit

Environment:
  SIFT_DEFAULT_COMMAND   command run to produce input when stdin is a terminal
  SIFT_DEFAULT_OPTS      default options prepended to the command line

Keys:
  Up / Ctrl-P, Down / Ctrl-N   move cursor      PgUp / PgDn   page
  Enter                        accept           Esc / Ctrl-C  cancel
  Tab                          mark item (with --multi)
  Ctrl-U clear query     Ctrl-W delete word     Backspace delete char
`

type config struct {
	query       string
	prompt      string
	multi       bool
	preview     string
	header      string
	headerLines int
	filter      string
	hasFilter   bool
	read0       bool
	print0      bool
	printQuery  bool
	expect      []string
	exact       bool
	caseMode    pattern.Case
	literal     bool
	noSort      bool
	tac         bool
	selectOne   bool
	exitZero    bool
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

// splitArgs performs a minimal shell-like split (honouring single/double
// quotes) of $SIFT_DEFAULT_OPTS.
func splitArgs(s string) []string {
	var args []string
	var cur strings.Builder
	inSingle, inDouble, has := false, false, false
	flush := func() {
		if has {
			args = append(args, cur.String())
			cur.Reset()
			has = false
		}
	}
	for _, r := range s {
		switch {
		case inSingle:
			if r == '\'' {
				inSingle = false
			} else {
				cur.WriteRune(r)
			}
		case inDouble:
			if r == '"' {
				inDouble = false
			} else {
				cur.WriteRune(r)
			}
		case r == '\'':
			inSingle, has = true, true
		case r == '"':
			inDouble, has = true, true
		case r == ' ' || r == '\t' || r == '\n':
			flush()
		default:
			cur.WriteRune(r)
			has = true
		}
	}
	flush()
	return args
}

func parseArgs(args []string) (config, error) {
	c := config{prompt: "> ", caseMode: pattern.CaseSmart}
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
		var v string
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
		case "--header-lines":
			if v, err = next(&i, a); err == nil {
				if _, e := fmt.Sscanf(v, "%d", &c.headerLines); e != nil || c.headerLines < 0 {
					err = fmt.Errorf("invalid value for --header-lines: %q", v)
				}
			}
		case "-f", "--filter":
			c.filter, err = next(&i, a)
			c.hasFilter = true
		case "-e", "--exact":
			c.exact = true
		case "-i", "--ignore-case":
			c.caseMode = pattern.CaseIgnore
		case "+i", "--no-ignore-case":
			c.caseMode = pattern.CaseRespect
		case "--literal":
			c.literal = true
		case "+s", "--no-sort":
			c.noSort = true
		case "--tac":
			c.tac = true
		case "-1", "--select-1":
			c.selectOne = true
		case "-0", "--exit-0":
			c.exitZero = true
		case "--print-query":
			c.printQuery = true
		case "--expect":
			if v, err = next(&i, a); err == nil {
				for _, k := range strings.Split(v, ",") {
					if k = strings.TrimSpace(k); k != "" {
						c.expect = append(c.expect, k)
					}
				}
			}
		case "--read0":
			c.read0 = true
		case "--print0":
			c.print0 = true
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

func reverse(s []string) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}

func main() {
	args := os.Args[1:]
	if def := os.Getenv("SIFT_DEFAULT_OPTS"); strings.TrimSpace(def) != "" {
		args = append(splitArgs(def), args...)
	}

	cfg, err := parseArgs(expandArgs(args))
	if err != nil {
		fail(err)
	}

	items, err := reader.Read(cfg.read0)
	if err != nil {
		fail(err)
	}

	// Peel off sticky header lines from the top of the input.
	var header []string
	if cfg.header != "" {
		header = append(header, strings.Split(cfg.header, "\n")...)
	}
	if cfg.headerLines > 0 {
		n := cfg.headerLines
		if n > len(items) {
			n = len(items)
		}
		header = append(header, items[:n]...)
		items = items[n:]
	}

	if cfg.tac {
		reverse(items)
	}

	sep := "\n"
	if cfg.print0 {
		sep = "\x00"
	}
	mopts := matcher.Options{Fuzzy: !cfg.exact, Case: cfg.caseMode, Sort: !cfg.noSort}

	out := bufio.NewWriter(os.Stdout)
	defer out.Flush()
	emit := func(s string) { out.WriteString(s); out.WriteString(sep) }

	// Non-interactive filter mode.
	if cfg.hasFilter {
		matches := matcher.Filter(items, cfg.filter, mopts)
		if cfg.printQuery {
			emit(cfg.filter)
		}
		for _, mt := range matches {
			emit(mt.Text)
		}
		out.Flush()
		if len(matches) == 0 {
			os.Exit(1)
		}
		return
	}

	// --select-1 / --exit-0: resolve trivial cases without showing the UI.
	if cfg.selectOne || cfg.exitZero {
		matches := matcher.Filter(items, cfg.query, mopts)
		if cfg.exitZero && len(matches) == 0 {
			if cfg.printQuery {
				emit(cfg.query)
			}
			out.Flush()
			os.Exit(1)
		}
		if cfg.selectOne && len(matches) == 1 {
			if cfg.printQuery {
				emit(cfg.query)
			}
			if len(cfg.expect) > 0 {
				emit("")
			}
			emit(matches[0].Text)
			return
		}
	}

	res, err := ui.Run(items, ui.Options{
		Prompt:  cfg.prompt,
		Query:   cfg.query,
		Multi:   cfg.multi,
		Preview: cfg.preview,
		Header:  header,
		Expect:  cfg.expect,
		Fuzzy:   !cfg.exact,
		Case:    cfg.caseMode,
		Sort:    !cfg.noSort,
	})
	if err != nil {
		fail(err)
	}

	if res.Aborted {
		if cfg.printQuery {
			emit(res.Query)
		}
		out.Flush()
		os.Exit(130)
	}

	if cfg.printQuery {
		emit(res.Query)
	}
	if len(cfg.expect) > 0 {
		emit(res.Key)
	}
	for _, s := range res.Selected {
		emit(s)
	}
}
