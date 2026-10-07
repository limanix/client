{ config, lib, ... }:
let
  inherit
    (config.lmx.capabilities.theme
      or (throw "The LimaNix platform needs the theme capability of module catalog v3 or newer.")
    )
    palette
    ;
  # A theme color such as "#89b4fa" as the "137;180;250" of a 24-bit terminal color.
  rgb =
    color:
    lib.concatMapStringsSep ";" (
      start: toString (lib.fromHexString (builtins.substring start 2 color))
    ) [ 1 3 5 ];
in
{
  # Keep the base prompt available without requiring a prompt or shell module.
  programs.bash.promptInit = lib.mkIf (!config.programs.starship.enable) (
    lib.mkDefault ''
      if [ -n "''${NO_COLOR:-}" ] || [ "''${TERM:-dumb}" = dumb ]; then
        PS1='\u@\h \w \$ '
      else
        PS1='\[\e[38;2;${rgb palette.blue}m\]\u@\h\[\e[0m\] \[\e[38;2;${rgb palette.green}m\]\w\[\e[0m\] \$ '
      fi
    ''
  );
}
