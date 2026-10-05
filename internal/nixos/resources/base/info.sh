# The summary and absolute command paths are supplied by workspace.nix.
# shellcheck disable=SC2154
if [ "$#" -ne 0 ]; then
  printf '%s\n' 'Usage: lmx info' >&2
  exit 2
fi

if ! "$limanix_cat" -- "$limanix_summary"; then
  printf '%s\n' 'Workspace metadata cannot be read.' >&2
  exit 1
fi
printf '\nKernel: '
"$limanix_uname" -sr || exit 1
printf '\nShared mounts (target, type, options)\n'
mounts=$("$limanix_findmnt" --kernel --list --noheadings --types virtiofs,9p --output TARGET,FSTYPE,OPTIONS)
mount_status=$?
case "$mount_status" in
  0) printf '%s\n' "$mounts" ;;
  1) printf '%s\n' '(none)' ;;
  *) printf '%s\n' 'Shared mounts could not be listed.' >&2; exit "$mount_status" ;;
esac
printf '\nFailed system services\n'
"$limanix_systemctl" --failed --no-pager --no-legend || exit 1
