# sift shell integration for zsh.
# Enable with:   eval "$(sift --zsh)"   in your ~/.zshrc

# CTRL-T - paste selected file/dir paths onto the command line.
__sift_files() {
  local cmd="${SIFT_CTRL_T_COMMAND:-find . -mindepth 1 \( -type f -o -type d \) 2>/dev/null | sed 's|^\./||'}"
  eval "$cmd" | sift --multi --prompt 'Files> '
}
sift-file-widget() {
  local result
  result=$(__sift_files | tr '\n' ' ')
  LBUFFER="${LBUFFER}${result% }"
  zle reset-prompt
}
zle -N sift-file-widget
bindkey '^T' sift-file-widget

# CTRL-R - search command history.
sift-history-widget() {
  local selected
  selected=$(fc -rl 1 | sed 's/^ *[0-9]* *//' | sift --prompt 'History> ' --query "$LBUFFER")
  [[ -n "$selected" ]] && LBUFFER="$selected"
  zle reset-prompt
}
zle -N sift-history-widget
bindkey '^R' sift-history-widget

# ALT-C - cd into a selected subdirectory.
sift-cd-widget() {
  local dir
  dir=$(find . -mindepth 1 -type d 2>/dev/null | sed 's|^\./||' | sift --prompt 'Dir> ')
  [[ -n "$dir" ]] && builtin cd -- "$dir"
  zle reset-prompt
}
zle -N sift-cd-widget
bindkey '\ec' sift-cd-widget
