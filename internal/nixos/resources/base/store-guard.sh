# Command paths and thresholds are supplied by platform.nix.
# shellcheck shell=bash disable=SC2154
# Keeps free bytes and inodes on the Nix store file system: below the collection threshold it removes
# unreferenced store paths; below the minimum it reports the garbage-collector roots that keep paths alive.

read_usage() {
  read -r blocks free_blocks inodes free_inodes \
    < <("$limanix_stat" --file-system --format='%b %f %c %d' /nix/store) || return 1
  case "$blocks$free_blocks$inodes$free_inodes" in
    '' | *[!0-9]*) return 1 ;;
  esac
}

# A file system that reports no inode table, such as btrfs, has no inode limit.
below() {
  { [ "$blocks" -gt 0 ] && [ $((free_blocks * 100)) -lt $((blocks * $1)) ]; } \
    || { [ "$inodes" -gt 0 ] && [ $((free_inodes * 100)) -lt $((inodes * $1)) ]; }
}

if ! read_usage; then
  printf '%s\n' 'Store file-system usage cannot be read.' >&2
  exit 1
fi
below "$limanix_collect_percent" || exit 0

printf 'Less than %s%% of the guest disk is free; collecting unreferenced store paths.\n' "$limanix_collect_percent"
"$limanix_nix_store" --gc --quiet || exit 1

if ! read_usage; then
  printf '%s\n' 'Store file-system usage cannot be read.' >&2
  exit 1
fi
below "$limanix_minimum_percent" || exit 0

# Roots of the running and applied system are expected; the rest are user builds, shells and profiles.
printf 'Less than %s%% of the guest disk is still free. Other garbage-collector roots:\n' "$limanix_minimum_percent"
"$limanix_nix_store" --gc --print-roots \
  | "$limanix_grep" -E -v -e '^"?/proc/' -e '^"?/run/' -e '^"?/nix/var/nix/profiles/system' -e '\{censored\}'
exit 0
