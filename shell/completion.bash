# sift fuzzy completion for bash.
#
# Type the trigger (** by default) and press TAB to fuzzy-complete paths, e.g.
#   vim **<TAB>      cat **<TAB>      cd **<TAB>
#
# Customise:
#   SIFT_COMPLETION_TRIGGER   the trigger sequence (default: **)
#   SIFT_COMPLETION_COMMAND   command producing candidates

SIFT_COMPLETION_TRIGGER="${SIFT_COMPLETION_TRIGGER:-**}"

_sift_complete() {
  local cur trigger
  cur="${COMP_WORDS[COMP_CWORD]}"
  trigger="$SIFT_COMPLETION_TRIGGER"

  if [[ "$cur" == *"$trigger" ]]; then
    local base="${cur%$trigger}"
    local sel
    sel=$( (eval "${SIFT_COMPLETION_COMMAND:-find . -mindepth 1 2>/dev/null | sed 's|^\./||'}") \
            | sift --reverse --query "$base" )
    if [[ -n "$sel" ]]; then
      COMPREPLY=( "$sel" )
    fi
    return 0
  fi

  # Otherwise fall back to standard filename completion.
  COMPREPLY=( $(compgen -f -- "$cur") )
}

# Enable for some common commands. Add your own with:
#   complete -F _sift_complete -o bashdefault -o default <command>
complete -F _sift_complete -o bashdefault -o default vim vi nvim nano emacs cat less tail head cd cp mv rm ls 2>/dev/null
