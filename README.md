<div align="center">

# sift

**A fast, interactive command-line fuzzy finder.**

[![CI](https://github.com/Anshika2203/sift/actions/workflows/ci.yml/badge.svg)](https://github.com/Anshika2203/sift/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/Anshika2203/sift.svg)](https://pkg.go.dev/github.com/Anshika2203/sift)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

</div>

`sift` takes any list of lines — files, command history, git branches, anything —
and lets you narrow it down to the one you want by typing just a few letters.
The characters you type only have to appear *in order*, not next to each other,
so `rprt` finds `report_2024_final.txt`. Matches are ranked so the most likely
result floats to the top.

```text
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
  consecutive runs are rewarded; long gaps are penalised.
- **Extended search syntax** — combine fuzzy, `'exact`, `^prefix`, `suffix$`,
  and `!inverse` terms in one query.
- **Fast** — matching is parallelised across every CPU core.
- **Preview window** — show file contents, a `git diff`, anything, for the
  highlighted item.
- **Multi-select** — mark several items with <kbd>Tab</kbd>.
- **Shell key-bindings** — <kbd>Ctrl-T</kbd> (files), <kbd>Ctrl-R</kbd>
  (history), <kbd>Alt-C</kbd> (cd) for bash, zsh, and fish.
- **Single static binary** — no runtime dependencies; trivial to distribute.

## Installation

### From source

Requires [Go](https://go.dev/dl/) 1.25 or newer (older toolchains are fetched
automatically by `go build`).

```sh
go install github.com/Anshika2203/sift@latest
```

…or clone and build:

```sh
git clone https://github.com/Anshika2203/sift.git
cd sift
make build          # produces ./sift (sift.exe on Windows); or: go build -o sift .
```

### Via package managers

**Homebrew** (macOS/Linux):

```sh
brew install --cask Anshika2203/tap/sift
# or tap once, then use the short name:
#   brew tap Anshika2203/tap
#   brew install --cask sift
```

**Scoop** (Windows):

```sh
scoop bucket add Anshika2203 https://github.com/Anshika2203/scoop-bucket
scoop install sift
```

**Debian/Ubuntu, Fedora/RHEL, Alpine** — grab the package from the
[latest release](https://github.com/Anshika2203/sift/releases/latest):

```sh
sudo dpkg -i sift_*_linux_amd64.deb                    # Debian/Ubuntu
sudo rpm  -i sift_*_linux_amd64.rpm                    # Fedora/RHEL
sudo apk add --allow-untrusted sift_*_linux_amd64.apk  # Alpine
```

> **Why not a bare `brew install sift`?** The short form (like `brew install fzf`)
> only works for tools accepted into Homebrew's official **homebrew-core** catalog,
> which is curated by Homebrew's maintainers and requires a project to be notable
> and well-established — you can't self-publish there. A personal tap always uses
> the `owner/tap/` prefix (or a one-time `brew tap`). Once `sift` gains traction it
> can be submitted to homebrew-core, after which `brew install sift` would work for
> everyone. (Scoop, by contrast, lets you use the bare `scoop install sift` as soon
> as the bucket is added.)

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

## Search syntax

By default a query is a **fuzzy** match. Separate the query with spaces to add
more terms — every term must match (logical AND). Markers change how a term
matches:

| Token | Match type | Example | Matches |
| --- | --- | --- | --- |
| `foo` | fuzzy | `fbb` | `FooBarBaz` |
| `'foo` | exact substring | `'bar` | `foo**bar**baz` |
| `^foo` | prefix | `^main` | `main.go` |
| `foo$` | suffix | `.go$` | `main.go` |
| `^foo$` | exact equality | `^README.md$` | `README.md` |
| `!foo` | inverse (must **not** match) | `!test` | excludes `main_test.go` |

The inverse marker combines with the others: `!^foo`, `!foo$`, `!'foo`.

```sh
# files containing "main", but not "test"
find . | sift --query 'main !test'
```

Matching is case-insensitive unless a term contains an uppercase letter
(*smart case*).

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

```text
input  ──▶  reader  ──▶  matcher  ──▶  ui
(stdin or          (lines)   (ranked    (interactive
 file walk)                   matches)    list + preview)
```

- **`internal/algo`** — the match + scoring engine. Fuzzy matching uses a greedy
  two-pass aligner (forward scan to locate the match, backward scan to tighten
  it) followed by a linear scoring pass that applies the boundary / camelCase /
  consecutive bonuses and gap penalties. The same scoring pass backs the
  exact / prefix / suffix / equal modes.
- **`internal/pattern`** — parses the extended search syntax into terms and
  evaluates them against an item.
- **`internal/matcher`** — runs the pattern across every item in parallel and
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

## Releasing

Releases are produced with [GoReleaser](https://goreleaser.com) (config in
`.goreleaser.yml`), which builds the binaries and the Homebrew / Scoop / deb /
rpm / apk artifacts:

```sh
git tag v0.1.0
git push --tags
GITHUB_TOKEN=... goreleaser release --clean
```

> The `brews:` and `scoops:` steps publish to `Anshika2203/homebrew-tap` and
> `Anshika2203/scoop-bucket`; create those repos first, or comment the sections
> out to skip them.

## License

[MIT](LICENSE)
