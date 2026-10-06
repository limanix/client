# limanix_summary and limanix_cat are supplied by workspace.nix.
# shellcheck disable=SC2154
if [ "$#" -gt 1 ] || { [ "$#" -eq 1 ] && [ "$1" != --help ] && [ "$1" != -h ]; }; then
  printf '%s\n' 'Usage: lmx help [--help]' >&2
  exit 2
fi

printf '%s\n\n' 'LimaNix workspace'
if ! "$limanix_cat" -- "$limanix_summary"; then
  printf '%s\n' 'Workspace metadata cannot be read.' >&2
  exit 1
fi
"$limanix_cat" <<'HELP'

Inside this VM
  lmx info              Show the kernel, guest disk, shared mounts and failed services.
  lmx welcome           Show the workspace welcome again.
  limanix-session NAME  Open a named session with the selected session provider.
  exit                  Return to the Mac.

On the Mac
  limanix list                          Show VM state and network addresses.
  limanix shell NAME                    Open the guest user's login shell.
  limanix shell NAME --session PROJECT  Open a named project session.
  limanix update --config FILE          Apply resources, modules and environment.
  limanix stop NAME                     Stop the VM; keep its disk and home.

Tools come from the modules selected in your TOML configuration.
No session provider is required for a normal shell.
Guide: https://limanix.dev/categories/client/getting-started.html
HELP
