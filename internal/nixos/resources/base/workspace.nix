{
  config,
  lib,
  pkgs,
  ...
}:
let
  runtime = builtins.fromJSON (builtins.readFile ./runtime.json);
  summary = pkgs.writeText "limanix-workspace" ''
    VM: ${runtime.name} (${runtime.arch})
    User: ${runtime.user.name}
    Home: ${runtime.user.home}
    Modules: ${if runtime.selectedModules == [ ] then "none" else lib.concatStringsSep ", " runtime.selectedModules}
  '';
  help = pkgs.writeShellScriptBin "limanix-help" ''
    limanix_summary=${lib.escapeShellArg (toString summary)}
    limanix_cat=${pkgs.coreutils}/bin/cat
    ${builtins.readFile ./help.sh}
  '';
  info = pkgs.writeShellScriptBin "limanix-info" ''
    limanix_summary=${lib.escapeShellArg (toString summary)}
    limanix_cat=${pkgs.coreutils}/bin/cat
    limanix_uname=${pkgs.coreutils}/bin/uname
    limanix_findmnt=${pkgs.util-linux}/bin/findmnt
    limanix_systemctl=${pkgs.systemd}/bin/systemctl
    ${builtins.readFile ./info.sh}
  '';
in
{
  environment.systemPackages = [ help info ];
  environment.etc."limanix/workspace".source = summary;

  environment.interactiveShellInit = ''
    if [ -t 1 ] && [ -z "''${LIMANIX_WELCOME_SHOWN:-}" ] \
      && [ "$(${pkgs.coreutils}/bin/id -u)" = "${toString runtime.user.uid}" ]; then
      export LIMANIX_WELCOME_SHOWN=1
      printf '\nLimanix · %s\n' ${lib.escapeShellArg runtime.name}
      printf '%s\n\n' 'Run limanix-help for workspace commands.'
    fi
  '';

  # Keep the base prompt available without requiring a prompt or shell module.
  programs.bash.promptInit = lib.mkIf (!config.programs.starship.enable) (lib.mkDefault ''
    if [ -n "''${NO_COLOR:-}" ] || [ "''${TERM:-dumb}" = dumb ]; then
      PS1='\u@\h \w \$ '
    else
      PS1='\[\e[38;2;137;180;250m\]\u@\h\[\e[0m\] \[\e[38;2;166;227;161m\]\w\[\e[0m\] \$ '
    fi
  '');
}
