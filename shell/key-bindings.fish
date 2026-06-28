# sift shell integration for fish.
# Enable with:   sift --fish | source   in your ~/.config/fish/config.fish

function __sift_files
    if set -q SIFT_CTRL_T_COMMAND
        eval $SIFT_CTRL_T_COMMAND | sift --multi --prompt 'Files> '
    else
        find . -mindepth 1 \( -type f -o -type d \) 2>/dev/null | sed 's|^\./||' | sift --multi --prompt 'Files> '
    end
end

function sift-file-widget
    set -l result (__sift_files | string join ' ')
    commandline -i -- $result
    commandline -f repaint
end

function sift-history-widget
    set -l result (history | sift --prompt 'History> ' --query (commandline))
    test -n "$result"; and commandline -- $result
    commandline -f repaint
end

function sift-cd-widget
    set -l dir (find . -mindepth 1 -type d 2>/dev/null | sed 's|^\./||' | sift --prompt 'Dir> ')
    test -n "$dir"; and cd $dir
    commandline -f repaint
end

bind \ct sift-file-widget
bind \cr sift-history-widget
bind \ec sift-cd-widget
