#!/bin/sh
# Fails when a package listed in nixos.packages provides a command that another package of the
# system also provides from a different store path. LISTED holds NAME<TAB>DIRECTORY lines and
# OTHERS DIRECTORY lines; the bin/ of a directory holds the commands of its package. Conflicts
# between packages that are not listed are left alone.
#
# Usage: packages-check.sh LISTED OTHERS
set -eu

tab=$(printf '\t')

# Prints KIND<TAB>NAME<TAB>DIRECTORY<TAB>COMMAND for every command of the packages in FILE.
commands() {
  kind=$1
  while IFS="$tab" read -r first second; do
    if [ "$kind" = listed ]; then
      name=$first
      directory=$second
    else
      directory=$first
      name=${directory##*/}
      name=${name#*-}
    fi
    [ -d "$directory/bin" ] || continue
    for command in "$directory"/bin/*; do
      [ -e "$command" ] || [ -L "$command" ] || continue
      printf '%s\t%s\t%s\t%s\n' "$kind" "$name" "$directory" "${command##*/}"
    done
  done <"$2"
}

{
  commands other "$2"
  commands listed "$1"
} | awk -F "$tab" '
  {
    kind = $1; name = $2; directory = $3; command = $4
    if (!(command in provider)) {
      provider[command] = directory; label[command] = name; origin[command] = kind
      next
    }
    if (kind != "listed" || provider[command] == directory) next
    if (origin[command] == "listed")
      printf "nixos.packages: %s and %s both provide %s; keep one, or use a module that supports several versions\n", label[command], name, command
    else
      printf "nixos.packages: %s provides %s, which %s (from a module or the platform) also provides; remove it from packages, or choose one in a NixOS module\n", name, command, label[command]
    failed = 1
  }
  END { exit failed }
' >&2
