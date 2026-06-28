# Record an animated demo of sift with VHS (https://github.com/charmbracelet/vhs).
#
#   1. Install VHS:            brew install vhs   (or: go install github.com/charmbracelet/vhs@latest)
#   2. Make sure `sift` is on your PATH.
#   3. From the repo root:     vhs docs/demo.tape
#
# This writes docs/demo.gif, which the README will display.

Output docs/demo.gif

Set Shell bash
Set FontSize 18
Set Width 1100
Set Height 640
Set Padding 24
Set Theme "Catppuccin Mocha"

Type "find . -type f | sift --preview 'cat {}'"
Enter
Sleep 1s

Type "rprt"
Sleep 1500ms

Down
Sleep 700ms
Down
Sleep 700ms
Up
Sleep 700ms

Enter
Sleep 1500ms
