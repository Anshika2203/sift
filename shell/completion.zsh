# sift fuzzy completion for zsh.
#
# Type the trigger (** by default) and press TAB to fuzzy-complete paths, e.g.
#   vim **<TAB>      cat **<TAB>      cd **<TAB>
#
# Customise:
#   SIFT_COMPLETION_TRIGGER   the trigger sequence (default: **)
#   SIFT_COMPLETION_COMMAND   command producing candidates

SIFT_COMPLETION_TRIGGER="${SIFT_COMPLETION_TRIGGER:-**}"

_sift_complete_widget() {
  local trigger="$SIFT_COMPLETION_TRIGGER"
  if [[ "$LBUFFER" == *"$trigger" ]]; then
    local base="${LBUFFER%$trigger}"
    local word="${base##* }"
    local sel
    sel=$( (eval "${SIFT_COMPLETION_COMMAND:-find . -mindepth 1 2>/dev/null | sed 's|^\./||'}") \
            | sift --reverse --query "$word" )
    if [[ -n "$sel" ]]; then
      if [[ "$base" == *" "* ]]; then
        LBUFFER="${base% *} $sel"
      else
        LBUFFER="$sel"
      fi
    fi
    zle reset-prompt
    return
  fi
  zle expand-or-complete
}

zle -N _sift_complete_widget
bindkey '^I' _sift_complete_widget
