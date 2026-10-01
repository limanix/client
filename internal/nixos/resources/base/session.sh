if [ "$#" -ne 1 ] || [ -z "$1" ]; then
  printf '%s\n' 'Usage: limanix-session NAME (one nonempty session name)' >&2
  exit 2
fi

if [ -z "$limanix_session_command" ]; then
  printf '%s\n' 'No session provider is configured in this VM.' >&2
  if [ -n "$limanix_session_providers" ]; then
    printf 'Select a session provider in nixos.modules: %s\n' "$limanix_session_providers" >&2
    printf '%s\n' 'Apply the configuration with limanix update --config <file> before reconnecting.' >&2
  fi
  exit 127
fi

exec "$limanix_session_command" "$1"
