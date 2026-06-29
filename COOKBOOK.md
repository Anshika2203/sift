# sift — Cookbook

Every feature of `sift`, with copy-paste examples for **Windows (PowerShell)**,
**macOS / Linux (Bash · Zsh)**, and **Fish**. Pick the column for your shell.

> **Binary name.** Examples use `sift`. If you haven't installed it yet and are
> running from the repo on Windows, use `.\sift.exe` instead of `sift`
> (or run `go install .` once so `sift` is on your PATH).
>
> **Preview commands run in a sub-shell**, not your interactive shell: `sh -c`
> on macOS/Linux and `cmd /c` on Windows. So preview snippets use `cat {}` /
> `ls {}` on Unix and `type {}` / `dir {}` on Windows — regardless of whether
> you use Bash, Zsh, or Fish.

---

## Table of contents

1. [Install & run](#1-install--run)
2. [Giving sift input](#2-giving-sift-input)
3. [Using what you selected](#3-using-what-you-selected)
4. [Search syntax (what you type)](#4-search-syntax-what-you-type)
5. [Matching & sorting options](#5-matching--sorting-options)
6. [Fields: --nth / --with-nth / --delimiter](#6-fields---nth----with-nth----delimiter)
7. [Preview window](#7-preview-window)
8. [Appearance & layout](#8-appearance--layout)
9. [Multi-select](#9-multi-select)
10. [Query history](#10-query-history)
11. [Scripting & automation](#11-scripting--automation)
12. [Environment variables](#12-environment-variables)
13. [Shell integration & key-bindings](#13-shell-integration--key-bindings)
14. [Fuzzy completion (`**<Tab>`)](#14-fuzzy-completion-tab)
15. [Real-world recipes](#15-real-world-recipes)
16. [Keys reference](#16-keys-reference)

---

## 1. Install & run

| | Command |
|---|---|
| **Any OS (Go)** | `go install github.com/Anshika2203/sift@latest` |
| **macOS / Linux (Homebrew)** | `brew install --cask Anshika2203/tap/sift` |
| **Windows (Scoop)** | `scoop bucket add Anshika2203 https://github.com/Anshika2203/scoop-bucket; scoop install sift` |
| **From source** | `git clone https://github.com/Anshika2203/sift && cd sift && go build -o sift .` |

Run it with no input and it lists files in the current directory:

```text
sift
```

---

## 2. Giving sift input

`sift` reads items from standard input, one per line.

**PowerShell (Windows)**
```powershell
Get-ChildItem -Recurse -File -Name | sift      # files under the current dir
Get-ChildItem -Name | sift                      # entries in the current dir
Get-ChildItem -Directory -Recurse -Name | sift  # directories only
1..100 | sift                                    # numbers
Get-Content big.log | sift                       # lines of a file
```

**Bash / Zsh (macOS / Linux)**
```bash
find . -type f | sift           # files
ls | sift                       # entries in the current dir
find . -type d | sift           # directories only
seq 100 | sift                  # numbers
cat big.log | sift              # lines of a file
rg --files | sift               # respect .gitignore (ripgrep)
fd --type f | sift              # respect .gitignore (fd)
```

**Fish**
```fish
find . -type f | sift
ls | sift
seq 100 | sift
```

NUL-separated input (for paths with newlines/spaces, e.g. `find -print0`):

```bash
find . -type f -print0 | sift --read0
```

---

## 3. Using what you selected

On <kbd>Enter</kbd>, `sift` prints your choice to stdout and exits. Capture it,
pipe it, or wrap it in command substitution.

**PowerShell**
```powershell
# capture in a variable
$file = Get-ChildItem -Recurse -File -Name | sift
$file

# open the picked file
code  (Get-ChildItem -Recurse -File -Name | sift)
notepad (Get-ChildItem -Recurse -File -Name | sift)

# cd into a picked directory
Set-Location (Get-ChildItem -Directory -Recurse -Name | sift)
```

**Bash / Zsh**
```bash
file=$(find . -type f | sift)
echo "$file"

# open in editor
${EDITOR:-vim} "$(find . -type f | sift)"
code "$(find . -type f | sift)"

# cd into a picked directory
cd "$(find . -type d | sift)"
```

**Fish**
```fish
set file (find . -type f | sift)
echo $file

cd (find . -type d | sift)
$EDITOR (find . -type f | sift)
```

---

## 4. Search syntax (what you type)

You type these *inside* `sift`. By default a query is **fuzzy** (characters in
order, gaps allowed). Separate terms with spaces — all terms must match (AND).

| Type this | Meaning | Example matches |
|---|---|---|
| `fbb` | fuzzy | `FooBarBaz` |
| `'bar` | exact substring | `foo`**`bar`**`baz` |
| `'wild'` | exact at word boundary | `a `**`wild`**` thing` (not `wildcard`) |
| `^src` | prefix | `src/main.go` |
| `.go$` | suffix | `main.go` |
| `^README.md$` | exact equality | `README.md` |
| `!test` | inverse (must NOT match) | excludes `main_test.go` |
| `!^src` `!.tmp$` `!'foo` | inverse prefix / suffix / exact | — |
| `go$ \| rb$` | OR | `main.go` **or** `app.rb` |

You can preview these without the UI using filter mode (`-f`):

```bash
# files ending in .go that are not tests
find . | sift -f 'go$ !test'

# Go or Ruby files
find . | sift -f 'go$ | rb$'
```

---

## 5. Matching & sorting options

```bash
sift -e                 # exact-match by default ('foo flips a term back to fuzzy)
sift -i                 # force case-insensitive
sift +i                 # force case-sensitive
                        # (default is smart-case: insensitive unless query has UPPER)
sift --algo v2          # optimal ranking (default)
sift --algo v1          # faster, greedy ranking
sift --tiebreak begin,length   # break score ties: length, begin, end, index
sift --no-sort          # keep input order (don't rank by score)
sift --tac              # reverse the input order (e.g. most-recent first)
```

**PowerShell**
```powershell
Get-ChildItem -Name | sift -e
Get-ChildItem -Name | sift --tiebreak "begin,length"
Get-Content history.txt | sift --no-sort --tac
```

---

## 6. Fields: --nth / --with-nth / --delimiter

Search or display only certain columns. Default delimiter is whitespace; set a
custom one with `-d`/`--delimiter`. Indexes are 1-based; negatives count from
the end; `2..`, `..3`, `2..4` are ranges.

**Bash / Zsh**
```bash
# search only the 2nd column (e.g. process name), keep the whole line
ps aux | sift --nth 11

# colon-delimited; search the first field (username)
sift -d ':' --nth 1 < /etc/passwd

# show only the last path component while searching it
find . -type f | sift --with-nth -1

# tab-delimited data, search columns 2 through 4
cat data.tsv | sift -d '\t' --nth 2..4
```

**PowerShell**
```powershell
# process list: search only the process-name column
Get-Process | Format-Table -HideTableHeaders ProcessName,Id,CPU | Out-String -Stream | sift --nth 1

# CSV: search the 2nd field
Get-Content people.csv | sift -d ',' --nth 2
```

---

## 7. Preview window

`--preview CMD` runs a command for the highlighted item and shows its output.
Placeholders expand (and are shell-quoted) in `CMD`.

| Placeholder | Expands to |
|---|---|
| `{}` | current item |
| `{q}` | current query |
| `{n}` | current item index |
| `{+}` | all selected items (or the current one) |
| `{1}` `{-1}` `{2..3}` | field(s) of the current item |

**PowerShell (preview runs in `cmd`)**
```powershell
Get-ChildItem -File -Name | sift --preview "type {}"
Get-ChildItem -Directory -Name | sift --preview "dir {}"
Get-ChildItem -Recurse -File -Name | sift --preview "type {}" --preview-window "down,40%"
```

**Bash / Zsh / Fish (preview runs in `sh`)**
```bash
find . -type f | sift --preview 'cat {}'
find . -type f | sift --preview 'head -100 {}'
find . -type f | sift --preview 'bat --color=always {}'   # if bat is installed
find . -type d | sift --preview 'ls -la {}'

# show a git diff for the picked file
git ls-files | sift --preview 'git diff --color=always -- {}'

# use the query and index in the preview
find . | sift --preview 'echo "query={q} index={n} item={}"'
```

Window placement / size / visibility:

```bash
sift --preview 'cat {}' --preview-window 'right,60%'
sift --preview 'cat {}' --preview-window 'up,50%'
sift --preview 'cat {}' --preview-window 'down,40%'
sift --preview 'cat {}' --preview-window 'left,30%'
sift --preview 'cat {}' --preview-window 'hidden'   # start hidden; Ctrl-O to show
```

In the finder: **Ctrl-O** toggles the preview, **Shift/Alt + ↑/↓** (or mouse
wheel) scroll it.

---

## 8. Appearance & layout

```bash
sift --reverse                       # top-down layout (prompt at top) — the default
sift --layout default                # bottom-up layout (prompt at the bottom)
sift --cycle                         # wrap around when moving past the ends
sift --no-mouse                      # disable mouse
sift --prompt 'pick> '               # custom prompt
sift --header 'Choose a file'        # sticky header line
sift --border rounded                # rounded border (also: sharp)
sift --margin 1                      # space around the finder
sift --padding '0,1'                 # space inside the border
sift --color 'prompt:cyan,hl:green,pointer:magenta,marker:yellow,info:blue,header:red'
```

Color element names: `prompt`, `pointer`, `marker`, `info`, `header`, `hl`,
`fg`. Colors: `black red green yellow blue magenta cyan white default` or a
number `0`–`255`.

**PowerShell — a fully decorated picker**
```powershell
Get-ChildItem -File -Name | sift `
  --preview "type {}" --preview-window "down,45%" `
  --border rounded --margin 1 --padding "0,1" `
  --prompt "file> " --header "Pick a file" `
  --color "hl:cyan,pointer:magenta,border:gray"
```

(PowerShell uses a backtick `` ` `` for line continuation; Bash/Zsh use `\`.)

---

## 9. Multi-select

`-m`/`--multi` lets you mark several items with <kbd>Tab</kbd> (un-mark with
<kbd>Tab</kbd> again, or <kbd>Shift-Tab</kbd>). <kbd>Enter</kbd> outputs all
marked items.

**Bash / Zsh**
```bash
# delete several files you pick
find . -type f | sift --multi | xargs rm -i

# stage several files in git
git ls-files -m | sift --multi | xargs git add
```

**PowerShell**
```powershell
# remove several picked files
Get-ChildItem -File -Name | sift --multi | ForEach-Object { Remove-Item $_ }

# copy several picked files somewhere
Get-ChildItem -File -Name | sift --multi | ForEach-Object { Copy-Item $_ C:\dest\ }
```

---

## 10. Query history

`--history FILE` loads past queries and saves new ones. Inside the finder,
**Ctrl-P** / **Ctrl-N** walk backward / forward through history (the arrow keys
still move the list).

**Bash / Zsh / Fish**
```bash
find . -type f | sift --history ~/.sift_history
```

**PowerShell**
```powershell
Get-ChildItem -Recurse -File -Name | sift --history "$env:USERPROFILE\.sift_history"
```

---

## 11. Scripting & automation

```bash
sift -f 'QUERY'         # non-interactive: print all matches and exit
sift -q 'QUERY'         # start interactive with an initial query
sift -1                 # if exactly one item matches, pick it without the UI
sift -0                 # if nothing matches, exit immediately (code 1)
sift --print-query      # print the final query as the first output line
sift --print0           # NUL-separate the output (pairs with xargs -0)
sift --expect ctrl-y,ctrl-e   # extra accept keys; the key name is printed first
```

Exit codes: `0` selected · `1` no match (filter / `-0`) · `2` usage error ·
`130` aborted (Esc / Ctrl-C).

**Bash / Zsh — react to which key was pressed**
```bash
out=$(find . -type f | sift --expect=ctrl-e,ctrl-v)
key=$(head -1 <<< "$out")
file=$(tail -1 <<< "$out")
case "$key" in
  ctrl-e) ${EDITOR:-vim} "$file" ;;
  ctrl-v) code "$file" ;;
  *)      echo "picked $file" ;;
esac
```

**Auto-pick when unambiguous**
```bash
# if exactly one file matches "config", open it without showing the UI
find . -type f | sift -q config -1 | xargs -r ${EDITOR:-vim}
```

**PowerShell**
```powershell
# non-interactive filter
Get-ChildItem -Recurse -File -Name | sift -f "main"

# capture query + selection with --expect
$out = Get-ChildItem -File -Name | sift --expect "ctrl-y"
$key  = $out[0]
$pick = $out[1]
```

---

## 12. Environment variables

| Variable | Effect |
|---|---|
| `SIFT_DEFAULT_COMMAND` | command run to produce input when stdin is a terminal |
| `SIFT_DEFAULT_OPTS` | default options prepended to every invocation |
| `SIFT_CTRL_T_COMMAND` | candidate command for the CTRL-T key-binding |
| `SIFT_COMPLETION_TRIGGER` | completion trigger (default `**`) |
| `SIFT_COMPLETION_COMMAND` | candidate command for `**<Tab>` completion |

**Bash / Zsh**
```bash
export SIFT_DEFAULT_COMMAND='fd --type f --hidden'
export SIFT_DEFAULT_OPTS='--reverse --border rounded --preview "cat {}"'
sift     # now uses fd for input and those default options
```

**Fish**
```fish
set -gx SIFT_DEFAULT_COMMAND 'fd --type f --hidden'
set -gx SIFT_DEFAULT_OPTS '--reverse --border rounded'
```

**PowerShell**
```powershell
$env:SIFT_DEFAULT_COMMAND = 'Get-ChildItem -Recurse -File -Name'
$env:SIFT_DEFAULT_OPTS    = '--reverse --border rounded'
sift
```

---

## 13. Shell integration & key-bindings

Loads <kbd>Ctrl-T</kbd> (paste files), <kbd>Ctrl-R</kbd> (history search), and
<kbd>Alt-C</kbd> (cd into a dir).

**Bash** — in `~/.bashrc`
```bash
eval "$(sift --bash)"
```

**Zsh** — in `~/.zshrc`
```zsh
eval "$(sift --zsh)"
```

**Fish** — in `~/.config/fish/config.fish`
```fish
sift --fish | source
```

**Windows PowerShell**: there is no PowerShell integration script yet — pipe
into `sift` directly (sections above). The key-bindings work under **WSL** or
**Git Bash** with the Bash snippet above.

---

## 14. Fuzzy completion (`**<Tab>`)

After loading the Bash or Zsh integration, type the trigger `**` and press
<kbd>Tab</kbd> to fuzzy-complete paths:

```bash
vim **<Tab>        # pick a file to edit
cat **<Tab>        # pick a file to cat
cd **<Tab>         # pick a directory
cp **<Tab> dest/   # pick a source file
```

Customize:
```bash
export SIFT_COMPLETION_TRIGGER='@@'                 # use @@ instead of **
export SIFT_COMPLETION_COMMAND='fd --type f'        # candidates for completion
```

---

## 15. Real-world recipes

### Browse files *and* folders with a smart preview

By default `Get-ChildItem -File` / `find -type f` hide folders. Drop the file
filter to include folders, and use a preview that shows a folder's contents or a
file's text depending on what's highlighted.

```powershell
# PowerShell — current folder (files + folders)
Get-ChildItem -Name | sift --preview "if exist {}\* (dir {}) else (type {})"

# PowerShell — recursive: jump to anything, including nested paths
Get-ChildItem -Recurse -Name | sift --preview "if exist {}\* (dir {}) else (type {})"

# tree view for folders
Get-ChildItem -Recurse -Name | sift --preview "if exist {}\* (tree /f {}) else (type {})"
```

```bash
# Bash / Zsh / Fish — folder listing or file text
find . | sift --preview '[ -d {} ] && ls -la {} || cat {}'

# nicer, if bat is installed: bat for files, ls for folders
find . | sift --preview '[ -d {} ] && ls -la --color=always {} || bat --color=always {}'
```

A fuzzy finder doesn't *open* a folder on Enter — list **recursively** and
fuzzy-jump to any nested path instead (type part of `dist\CHANGELOG.md` to land
on it), and highlight a folder to peek inside via the preview. Press-to-descend
navigation will arrive with the `--bind reload` action in a later release.

### Files → editor

```bash
# Bash / Zsh
${EDITOR:-vim} "$(find . -type f | sift --preview 'cat {}')"
```
```powershell
# PowerShell
code (Get-ChildItem -Recurse -File -Name | sift --preview "type {}")
```

### Git: switch branch

```bash
# Bash / Zsh
git branch --format='%(refname:short)' | sift | xargs -r git switch
```
```powershell
# PowerShell
git branch --format='%(refname:short)' | sift | ForEach-Object { git switch $_ }
```

### Git: pick files from `git status` and stage them

```bash
git -c color.status=always status --short | sift --ansi --multi --nth 2.. \
  | awk '{print $2}' | xargs -r git add
```

### Git: browse log and show a commit

```bash
git log --oneline --color=always | sift --ansi --preview 'git show --color=always {1}' \
  | awk '{print $1}' | xargs -r git show
```

### Kill a process

```bash
# Bash / Zsh
ps -ef | sift --header-lines 1 --multi | awk '{print $2}' | xargs -r kill
```
```powershell
# PowerShell
Get-Process | Format-Table -HideTableHeaders Id,ProcessName | Out-String -Stream `
  | Where-Object { $_.Trim() } | sift | ForEach-Object { Stop-Process -Id ($_ -split '\s+')[0] }
```

### cd into any subdirectory

```bash
# Bash / Zsh
cd "$(find . -type d | sift)"
```
```fish
cd (find . -type d | sift)
```
```powershell
Set-Location (Get-ChildItem -Directory -Recurse -Name | sift)
```

### Search your shell history

```bash
# Bash
history | sed 's/^ *[0-9]* *//' | sift --tac --no-sort | tail -1
```
```powershell
# PowerShell (PSReadLine history file)
Get-Content (Get-PSReadLineOption).HistorySavePath | sift --tac --no-sort
```

### Pick an environment variable

```bash
# Bash / Zsh
printenv | sift -d '=' --nth 1
```
```powershell
# PowerShell
Get-ChildItem Env: | ForEach-Object { "$($_.Name)=$($_.Value)" } | sift -d '=' --nth 1
```

### Preview with line numbers / syntax highlight (if `bat` is installed)

```bash
find . -type f | sift --preview 'bat --style=numbers --color=always {}' --preview-window 'right,60%'
```

### A single, fully-loaded everyday picker

```bash
# Bash / Zsh
find . -type f | sift \
  --multi --reverse --cycle \
  --border rounded --margin 1 --padding '0,1' \
  --prompt 'files> ' --header 'Tab: mark · Enter: open · Ctrl-O: toggle preview' \
  --preview 'cat {}' --preview-window 'right,55%' \
  --color 'hl:green,pointer:magenta,marker:yellow'
```

---

### Custom key bindings (`--bind`)

`--bind 'KEY:ACTION[+ACTION...]'` (repeatable) maps keys/events to actions like
`accept` `abort` `toggle` `select-all` `jump` `backward` `reload(..)`
`execute(..)` `become(..)`, plus events `start` `change` `focus`. Placeholders
`{} {q} {n} {+} {1}` expand in command actions.

> On Windows the action commands run in **cmd**; on macOS/Linux in **sh**.

```powershell
# PowerShell — open the highlighted file (become), or peek with execute
Get-ChildItem -Recurse -File -Name | sift --bind "enter:become(notepad {})"
Get-ChildItem -File -Name | sift --bind "ctrl-e:execute(more {})" --preview "type {}"

# jump mode: press ctrl-j then a label key
Get-ChildItem -Name | sift --bind "ctrl-j:jump"

# folder navigation with go-back (preview stays working via absolute paths)
Get-ChildItem -Recurse | ForEach-Object FullName | sift `
  --preview "if exist {}\* (dir {}) else (type {})" `
  --bind "right:reload(dir /b /s {})" --bind "left:backward"
```

```bash
# Bash / Zsh — editor, live grep, folder navigation
find . -type f | sift --bind 'enter:become(${EDITOR:-vim} {})'
sift --bind 'change:reload(rg --line-number {q} || true)'
find . | sift --preview '[ -d {} ] && ls {} || cat {}' \
  --bind 'right:reload(find {} -maxdepth 1)' --bind 'left:backward'
```

Drive a running sift from another program with `--listen`:
```bash
sift --listen 6266 &        # then, from elsewhere:
curl -XPOST localhost:6266 -d 'reload(ls)'
curl -XPOST localhost:6266 -d 'change-query(foo)'
```

## 16. Keys reference

| Key | Action |
|---|---|
| Type text | filter (supports the [search syntax](#4-search-syntax-what-you-type)) |
| <kbd>↑</kbd> / <kbd>Ctrl-P</kbd>, <kbd>↓</kbd> / <kbd>Ctrl-N</kbd> | move cursor (`Ctrl-P/N` = history when `--history` is set) |
| <kbd>PgUp</kbd> / <kbd>PgDn</kbd> | page up / down |
| <kbd>Enter</kbd> | accept (print selection) |
| <kbd>Esc</kbd> / <kbd>Ctrl-C</kbd> | cancel |
| <kbd>Tab</kbd> / <kbd>Shift-Tab</kbd> | mark / unmark (with `--multi`) |
| <kbd>Ctrl-U</kbd> | clear query |
| <kbd>Ctrl-W</kbd> | delete word |
| <kbd>Backspace</kbd> | delete character |
| <kbd>Ctrl-O</kbd> | toggle preview window |
| <kbd>Shift</kbd>/<kbd>Alt</kbd> + <kbd>↑</kbd>/<kbd>↓</kbd> | scroll preview |
| Mouse wheel / click | scroll / select (unless `--no-mouse`) |

---

_For the option summary see `sift --help`; for the project overview see the
[README](README.md)._
