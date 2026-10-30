{ config, lib, ... }:
let
  inherit (config.lmx.capabilities) theme;
  # A theme color such as "#89b4fa" as the "137;180;250" of a 24-bit terminal color.
  rgb =
    color:
    lib.concatMapStringsSep ";" (
      start: toString (lib.fromHexString (builtins.substring start 2 color))
    ) [ 1 3 5 ];
  # The SGR parameters of a role: the terminal's own color with the system theme.
  sgr =
    role:
    let
      color = theme.colors.${role};
    in
    if builtins.isString color then
      "38;2;${rgb color}"
    else if color == null then
      "39"
    else if color < 8 then
      toString (30 + color)
    else
      toString (90 + color - 8);
in
{
  # Keep the base prompt available without requiring a prompt or shell module.
  programs.bash.promptInit = lib.mkIf (!config.programs.starship.enable) (
    lib.mkDefault ''
      if [ -n "''${NO_COLOR:-}" ] || [ "''${TERM:-dumb}" = dumb ]; then
        PS1='\u@\h \w \$ '
      else
        PS1='\[\e[${sgr "blue"}m\]\u@\h\[\e[0m\] \[\e[${sgr "green"}m\]\w\[\e[0m\] \$ '
      fi
    ''
  );
}
