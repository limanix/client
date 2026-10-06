# Command paths and the terminal device are supplied by workspace.nix.
# shellcheck shell=bash disable=SC2154
# Prints the Mac clipboard by asking the terminal with OSC 52; the terminal on the Mac must allow reads.
if [ "$#" -ne 0 ]; then
  printf '%s\n' 'Usage: pbpaste' >&2
  exit 2
fi

unanswered() {
  printf '%s\n' 'The terminal did not share its clipboard. Allow clipboard reads in its settings, or paste with Cmd+V.' >&2
  exit 1
}

# tmux asks the attached client's terminal and stores the answer as a new buffer.
if [ -n "${TMUX:-}" ] && command -v tmux >/dev/null; then
  latest() { tmux list-buffers -F '#{buffer_created} #{buffer_name}' 2>/dev/null | head -n 1; }
  before=$(latest)
  tmux refresh-client -l || exit 1
  # The terminal may ask for permission first; poll every 0.1 s until the timeout.
  attempts=$((limanix_paste_timeout * 10))
  while [ "$attempts" -gt 0 ]; do
    if [ "$(latest)" != "$before" ]; then
      exec tmux save-buffer -
    fi
    sleep 0.1
    attempts=$((attempts - 1))
  done
  unanswered
fi

if ! { exec 3<"$limanix_tty_in" 4>"$limanix_tty_out"; } 2>/dev/null; then
  printf '%s\n' 'pbpaste needs the terminal of a limanix shell session.' >&2
  exit 1
fi
mode=$("$limanix_stty" -g <&3) || exit 1
# The answer arrives as terminal input without a newline: read it raw, without echo.
"$limanix_stty" -echo -icanon <&3
printf '\033]52;c;?\a' >&4
IFS= read -r -d $'\a' -t "$limanix_paste_timeout" reply <&3
"$limanix_stty" "$mode" <&3

case "$reply" in
  $'\033]52;'*';'?*) printf '%s' "${reply##*;}" | "$limanix_base64" -d ;;
  *) unanswered ;;
esac
