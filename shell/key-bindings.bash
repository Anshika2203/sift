# sift shell integration for bash.
# Enable with:   eval "$(sift --bash)"   in your ~/.bashrc

if [[ $- == *i* ]]; then

  # CTRL-T - paste selected file/dir paths onto the command line.
  __sift_files() {
    local cmd="${SIFT_CTRL_T_COMMAND:-find . -mindepth 1 \( -type f -o -type d \) 2>/dev/null | sed 's|^\./||'}"
    eval "$cmd" | sift --multi --prompt 'Files> '
  }
  __sift_file_widget() {
    local result
    result=$(__sift_files | tr '\n' ' ')
    result=${result% }
    READLINE_LINE="${READLINE_LINE:0:$READLINE_POINT}${result}${READLINE_LINE:$READLINE_POINT}"
    READLINE_POINT=$(( READLINE_POINT + ${#result} ))
  }
  bind -m emacs-standard -x '"\C-t": __sift_file_widget' 2>/dev/null

  # CTRL-R - search command history.
  __sift_history_widget() {
    local result
    result=$(HISTTIMEFORMAT= history | sed 's/^ *[0-9]*[ *] *//' | sift --prompt 'History> ' --query "$READLINE_LINE")
    if [[ -n "$result" ]]; then
      READLINE_LINE="$result"
      READLINE_POINT=${#READLINE_LINE}
    fi
  }
  bind -m emacs-standard -x '"\C-r": __sift_history_widget' 2>/dev/null

  # ALT-C - cd into a selected subdirectory.
  __sift_cd_widget() {
    local dir
    dir=$(find . -mindepth 1 -type d 2>/dev/null | sed 's|^\./||' | sift --prompt 'Dir> ') || return
    [[ -n "$dir" ]] && builtin cd -- "$dir"
  }
  bind -m emacs-standard -x '"\ec": __sift_cd_widget' 2>/dev/null

fi
