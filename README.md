# sift

**A fast, interactive command-line fuzzy finder.**

`sift` takes any list of lines — files, command history, git branches, anything —
and lets you narrow it down to the one you want by typing just a few letters.
The characters you type only have to appear *in order*, not next to each other,
so `rprt` finds `report_2024_final.txt`. Matches are ranked so the most likely
result floats to the top.

```
> rprt
  2/4213
> report_2024_final.txt
  quarterly_report.txt
```

Pipe a list in, type to filter, press <kbd>Enter</kbd>, and `sift` prints what
you picked.

---

## Features

- **Fuzzy matching with smart ranking** — word boundaries, camelCase humps, and
  consecutive runs are all rewarded; long gaps are penalised.
- **Fast** — matching is parallelised across all CPU cores.
- **Preview window** — show file contents, `git diff`, anything, for the
  highlighted item.
- **Multi-select** — mark several items with <kbd>Tab</kbd>.
- **Shell key-bindings** — <kbd>Ctrl-T</kbd> (files), <kbd>Ctrl-R</kbd>
  (history), <kbd>Alt-C</kbd> (cd) for bash, zsh, and fish.
- **Single static binary** — no runtime dependencies; trivial to distribute.

## Installation

### From source (works today)

Requires [Go](https://go.dev/dl/) 1.25 or newer (older toolchains are fetched
automatically by `go build`).

```sh
git clone https://github.com/OWNER/sift.git
cd sift
go build -o sift .       # produces ./sift (or sift.exe on Windows)
```

Then move the binary somewhere on your `PATH`, e.g. `/usr/local/bin` (or run
`make install`).

### Via package managers (after you publish releases)

Once you tag a release and run [GoReleaser](https://goreleaser.com) (see
`.goreleaser.yml`), these become available:

| Platform | Command |
| --- | --- |
| Homebrew | `brew install OWNER/tap/sift` |
| Scoop (Windows) | `scoop install sift` |
| Debian/Ubuntu | `sudo dpkg -i sift_*.deb` |
| Fedora/RHEL | `sudo rpm -i sift-*.rpm` |

> Replace `OWNER` with your GitHub username throughout this file, the module
> path in `go.mod`, and `.goreleaser.yml` before publishing.

## Usage

```sh
# Pick a file
find . -type f | sift

# Pick a git branch and check it out
git branch | sed 's/^[* ] //' | sift | xargs git checkout

# Preview file contents while you browse
find . -type f | sift --preview 'cat {}'

# Multi-select with Tab
ls | sift --multi
```

With nothing piped in, `sift` lists files in the current directory:

```sh
sift
```

### Options

| Flag | Description |
| --- | --- |
| `-q`, `--query STR` | start with an initial query |
| `-p`, `--prompt STR` | set the prompt (default `> `) |
| `-m`, `--multi` | enable multi-select |
| `--preview CMD` | run `CMD` for the highlighted item; `{}` is the item |
| `--header STR` | show a fixed header line above the list |
| `-f`, `--filter STR` | non-interactive: print matches and exit |
| `--read0` | read NUL-separated input |
| `--bash` / `--zsh` / `--fish` | print the shell key-binding script |
| `-V`, `--version` | print version |
| `-h`, `--help` | show help |

### Keys

| Key | Action |
| --- | --- |
| <kbd>↑</kbd> / <kbd>Ctrl-P</kbd>, <kbd>↓</kbd> / <kbd>Ctrl-N</kbd> | move cursor |
| <kbd>PgUp</kbd> / <kbd>PgDn</kbd> | page up / down |
| <kbd>Enter</kbd> | accept selection |
| <kbd>Esc</kbd> / <kbd>Ctrl-C</kbd> | cancel |
| <kbd>Tab</kbd> | mark item (with `--multi`) |
| <kbd>Ctrl-U</kbd> | clear query |
| <kbd>Ctrl-W</kbd> | delete word |
| <kbd>Backspace</kbd> | delete character |

## Shell integration

Add the key-bindings to your shell to get <kbd>Ctrl-T</kbd>, <kbd>Ctrl-R</kbd>,
and <kbd>Alt-C</kbd>:

```sh
# bash — in ~/.bashrc
eval "$(sift --bash)"

# zsh — in ~/.zshrc
eval "$(sift --zsh)"

# fish — in ~/.config/fish/config.fish
sift --fish | source
```

## How it works

```
input  ──▶  reader  ──▶  matcher  ──▶  ui
(stdin or          (lines)   (ranked    (interactive
 file walk)                   matches)    list + preview)
```

- **`internal/algo`** — the fuzzy match + scoring engine. A greedy two-pass
  aligner (forward scan to locate the match, backward scan to tighten it)
  followed by a linear scoring pass that applies the boundary / camelCase /
  consecutive bonuses and gap penalties.
- **`internal/matcher`** — runs the algorithm across every item in parallel and
  sorts the results (score, then length, then input order).
- **`internal/reader`** — reads lines from stdin, or walks the filesystem when
  stdin is a terminal.
- **`internal/ui`** — the full-screen interactive interface (built on
  [tcell](https://github.com/gdamore/tcell)), including the async preview pane.

## Development

```sh
make build    # compile ./sift
make test     # run the test suite
make cross    # build release binaries for all platforms into dist/
```

## License

[MIT](LICENSE)
