# Script paths are supplied by workspace.nix.
# shellcheck disable=SC2154,SC1090
subcommand=${1:-help}
if [ "$#" -gt 0 ]; then
  shift
fi
case "$subcommand" in
  help | --help | -h) . "$limanix_help" ;;
  info) . "$limanix_info" ;;
  welcome) . "$limanix_welcome" ;;
  *)
    printf '%s\n' 'Usage: lmx {help|info|welcome}' >&2
    exit 2
    ;;
esac
