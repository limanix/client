# Metadata and absolute command paths are supplied by workspace.nix.
# shellcheck shell=bash disable=SC2154
if [ "$#" -ne 0 ]; then
  printf '%s\n' 'Usage: lmx welcome' >&2
  exit 2
fi

# Catppuccin Mocha, the palette of the fallback prompt.
blue='' mauve='' subtext='' muted='' green='' peach='' yellow='' bold='' reset=''
if [ -t 1 ] && [ -z "${NO_COLOR:-}" ] && [ "${TERM:-dumb}" != dumb ]; then
  blue=$'\e[38;2;137;180;250m'
  mauve=$'\e[38;2;203;166;247m'
  subtext=$'\e[38;2;166;173;200m'
  muted=$'\e[38;2;127;132;156m'
  green=$'\e[38;2;166;227;161m'
  peach=$'\e[38;2;250;179;135m'
  yellow=$'\e[38;2;249;226;175m'
  bold=$'\e[1m'
  reset=$'\e[0m'
fi

welcome_bytes() {
  local LC_ALL=C
  bytes=${#1}
}

# Terminal columns: one per character, two for characters of three or more
# UTF-8 bytes (CJK and emoji). Never less than the real width.
welcome_columns() {
  local LC_ALL=C.UTF-8 text=$1 i=0
  columns=${#text}
  welcome_bytes "$text"
  [ "$bytes" -ne "$columns" ] || return 0
  while [ "$i" -lt "${#text}" ]; do
    welcome_bytes "${text:i:1}"
    [ "$bytes" -lt 3 ] || columns=$((columns + 1))
    i=$((i + 1))
  done
}

# Wrap at a column width without splitting UTF-8 characters.
welcome_wrap() {
  local LC_ALL=C.UTF-8 text=$1 width=$2 part split columns
  while [ -n "$text" ]; do
    part=${text:0:width}
    welcome_columns "$part"
    while [ "$columns" -gt "$width" ]; do
      part=${part%?}
      welcome_columns "$part"
    done
    if [ "${#part}" -lt "${#text}" ] && [[ "$part" == *' '* ]]; then
      split=${part% *}
      [ -z "$split" ] || part=$split
    fi
    printf '%s\n' "$part"
    text=${text:${#part}}
    text=${text# }
  done
}

# Labels take 11 columns after the two-column indent; values wrap at 80.
welcome_row() {
  local label=$1 value=$2 color=${3:-} line
  while IFS= read -r line; do
    printf '  %s%-11s%s%s%s%s\n' "$muted" "$label" "$reset" "$color" "$line" "$reset"
    label=
  done < <(welcome_wrap "$value" 67)
}

# Kibibytes to a short size: one decimal below 10 GiB.
welcome_size() {
  local tenths=$((($1 * 10 + 524288) / 1048576))
  if [ "$tenths" -ge 100 ]; then
    size="$(((tenths + 5) / 10)) GiB"
  else
    size="$((tenths / 10)).$((tenths % 10)) GiB"
  fi
}

# Adds a limit below the platform minimum to short; unreadable values are left out.
welcome_short() {
  case "$1" in '' | *[!0-9]*) return 0 ;; esac
  case "$2" in '' | *[!0-9]*) return 0 ;; esac
  if [ "$2" -gt 0 ] && [ $(($1 * 100)) -lt $(($2 * limanix_minimum_percent)) ]; then
    short+=("$(($1 * 100 / $2))% of $3 free")
  fi
}

printf '\n'
while IFS= read -r line; do
  printf '  %s%s%s%s%s\n' "$blue" "${line:0:36}" "$mauve" "${line:36}" "$reset"
done <<'LOGO'
888      d8b                        888b    888 d8b
888      Y8P                        8888b   888 Y8P
888                                 88888b  888
888      888 88888b.d88b.   8888b.  888Y88b 888 888 888  888
888      888 888 "888 "88b     "88b 888 Y88b888 888 `Y8bd8P'
888      888 888  888  888 .d888888 888  Y88888 888   X88K
888      888 888  888  888 888  888 888   Y8888 888 .d8""8b.
88888888 888 888  888  888 "Y888888 888    Y888 888 888  888
LOGO
printf '\n'

if [ $((${#limanix_name} + 3 + ${#limanix_system})) -le 67 ]; then
  printf '  %s%-11s%s%s%s%s%s (%s)%s\n' "$muted" VM "$reset" "$bold" "$limanix_name" "$reset" \
    "$subtext" "$limanix_system" "$reset"
else
  welcome_row VM "$limanix_name"
  welcome_row '' "($limanix_system)" "$subtext"
fi
printf '\n'

# Resources are best effort: a part that cannot be read is left out.
resources=()
cpus=$("$limanix_nproc" 2>/dev/null)
case "$cpus" in
  1) resources+=('1 CPU') ;;
  '' | *[!0-9]*) ;;
  *) resources+=("$cpus CPUs") ;;
esac
memory=
{ while read -r key value _; do
  [ "$key" != MemTotal: ] || memory=$value
done < "$limanix_meminfo"; } 2>/dev/null
case "$memory" in
  '' | *[!0-9]*) ;;
  *) welcome_size "$memory"; resources+=("$size memory") ;;
esac
# The guest disk holds NixOS, the Nix store and service data; the home is a host mount.
disk= free=
{ read -r _ && read -r _ disk _ free _; } < <("$limanix_df" -Pk -- / 2>/dev/null)
case "$free" in
  '' | *[!0-9]*) ;;
  *) welcome_size "$free"; resources+=("$size disk free") ;;
esac
if [ "${#resources[@]}" -gt 0 ]; then
  summary=${resources[0]}
  for part in "${resources[@]:1}"; do
    summary+=", $part"
  done
  welcome_row Resources "$summary"
  printf '\n'
fi

welcome_row Modules "$limanix_modules"
printf '\n'

mounts=$("$limanix_findmnt" --kernel --list --json --types virtiofs,9p --output TARGET,OPTIONS)
mount_status=$?
label=Shared
case "$mount_status" in
  0)
    rows=$(printf '%s\n' "$mounts" | "$limanix_jq" -r '
      .filesystems[] | [.target, (if (.options | split(",") | index("ro")) then "ro" else "rw" end)] | @tsv
    ')
    mount_status=$?
    if [ "$mount_status" -eq 0 ]; then
      # Modes follow the longest target; longer targets wrap at 63 columns.
      width=0
      while IFS=$'\t' read -r target _; do
        welcome_columns "$target"
        [ "$columns" -le "$width" ] || width=$columns
      done <<< "$rows"
      [ "$width" -le 63 ] || width=63
      while IFS=$'\t' read -r target mode; do
        [ -n "$target" ] || continue
        color=$green
        [ "$mode" != ro ] || color=$peach
        while IFS= read -r line; do
          if [ -n "$mode" ]; then
            # Pad by columns: printf widths count bytes or characters, not columns.
            welcome_columns "$line"
            printf '  %s%-11s%s%s%*s  %s%s%s\n' "$muted" "$label" "$reset" "$line" \
              $((width - columns)) '' "$color" "$mode" "$reset"
          else
            printf '  %-11s%s\n' '' "$line"
          fi
          label='' mode=''
        done < <(welcome_wrap "$target" "$width")
      done <<< "$rows"
    fi
    ;;
  1) welcome_row Shared '(none)'; mount_status=0 ;;
esac
if [ "$mount_status" -ne 0 ]; then
  welcome_row Shared 'unavailable; run lmx info'
fi
printf '\n'

# A nearly full guest disk stops NixOS builds; ext4 can run out of inodes while space remains.
inodes= inodes_free=
{ read -r _ && read -r _ inodes _ inodes_free _; } < <("$limanix_df" -Pi -- / 2>/dev/null)
short=()
welcome_short "$free" "$disk" space
welcome_short "$inodes_free" "$inodes" inodes
if [ "${#short[@]}" -gt 0 ]; then
  detail=${short[0]}
  [ "${#short[@]}" -eq 1 ] || detail+=" and ${short[1]}"
  while IFS= read -r line; do
    printf '  %s%s%s\n' "$yellow" "$line" "$reset"
  done < <(welcome_wrap "▲ Guest disk nearly full: $detail. Run lmx info for details." 78)
  printf '\n'
fi

# Failed units are shown only when there are some.
failed=()
while read -r unit _; do
  [ -z "$unit" ] || failed+=("$unit")
done < <("$limanix_systemctl" --failed --no-legend --plain --no-pager 2>/dev/null)
if [ "${#failed[@]}" -gt 0 ]; then
  units=${failed[0]}
  for unit in "${failed[@]:1}"; do
    units+=", $unit"
  done
  while IFS= read -r line; do
    printf '  %s%s%s\n' "$yellow" "$line" "$reset"
  done < <(welcome_wrap "▲ Failed: $units. Run lmx info for details." 78)
  printf '\n'
fi

printf '  %slmx help%s%s for commands, %s%slmx info%s%s for details, %s%sexit%s%s to return to the Mac.%s\n\n' \
  "$blue" "$reset" "$muted" "$reset" "$blue" "$reset" "$muted" "$reset" "$blue" "$reset" "$muted" "$reset"
exit "$mount_status"
