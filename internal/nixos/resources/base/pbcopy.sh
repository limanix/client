# Command paths and the terminal device are supplied by workspace.nix.
# shellcheck shell=bash disable=SC2154
# Copies standard input to the Mac clipboard with OSC 52; the terminal on the Mac must allow it.
if [ "$#" -ne 0 ]; then
  printf '%s\n' 'Usage: pbcopy < FILE' >&2
  exit 2
fi

# tmux sends its buffer to the attached client's terminal, with or without the tmux module.
if [ -n "${TMUX:-}" ] && command -v tmux >/dev/null; then
  exec tmux load-buffer -w -
fi

data=$("$limanix_base64" -w0) || exit 1
if ! { printf '\033]52;c;%s\a' "$data" > "$limanix_tty_out"; } 2>/dev/null; then
  printf '%s\n' 'pbcopy needs the terminal of a limanix shell session.' >&2
  exit 1
fi
