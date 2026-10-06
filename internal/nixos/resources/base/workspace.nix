{
  config,
  lib,
  pkgs,
  ...
}:
let
  runtime = builtins.fromJSON (builtins.readFile ./runtime.json);
  modules =
    if runtime.selectedModules == [ ] then
      "none"
    else
      lib.concatStringsSep ", " runtime.selectedModules;
  summary = pkgs.writeText "limanix-workspace" ''
    VM: ${runtime.name} (${runtime.arch})
    User: ${runtime.user.name}
    Home: ${runtime.user.home}
    Modules: ${modules}
  '';
  lmx = pkgs.writeShellScriptBin "lmx" ''
    limanix_summary=${lib.escapeShellArg (toString summary)}
    limanix_cat=${pkgs.coreutils}/bin/cat
    limanix_jq=${pkgs.jq}/bin/jq
    limanix_uname=${pkgs.coreutils}/bin/uname
    limanix_findmnt=${pkgs.util-linux}/bin/findmnt
    limanix_systemctl=${pkgs.systemd}/bin/systemctl
    limanix_nproc=${pkgs.coreutils}/bin/nproc
    limanix_df=${pkgs.coreutils}/bin/df
    limanix_meminfo=/proc/meminfo
    limanix_name=${lib.escapeShellArg runtime.name}
    limanix_system=${lib.escapeShellArg "NixOS ${config.system.nixos.release}, ${runtime.arch}"}
    limanix_modules=${lib.escapeShellArg modules}
    limanix_minimum_percent=${toString runtime.disk.minimumPercent}
    limanix_help=${pkgs.writeText "lmx-help.sh" (builtins.readFile ./help.sh)}
    limanix_info=${pkgs.writeText "lmx-info.sh" (builtins.readFile ./info.sh)}
    limanix_welcome=${pkgs.writeText "lmx-welcome.sh" (builtins.readFile ./welcome.sh)}
    ${builtins.readFile ./lmx.sh}
  '';
in
{
  environment.systemPackages = [ lmx ];
  environment.etc."limanix/workspace".source = summary;

  environment.interactiveShellInit = ''
    if [ -t 1 ] && [ -z "''${LIMANIX_WELCOME_SHOWN:-}" ] \
      && [ "$(${pkgs.coreutils}/bin/id -u)" = "${toString runtime.user.uid}" ]; then
      export LIMANIX_WELCOME_SHOWN=1
      ${lmx}/bin/lmx welcome || :
    fi
  '';

  # Keep the base prompt available without requiring a prompt or shell module.
  programs.bash.promptInit = lib.mkIf (!config.programs.starship.enable) (
    lib.mkDefault ''
      if [ -n "''${NO_COLOR:-}" ] || [ "''${TERM:-dumb}" = dumb ]; then
        PS1='\u@\h \w \$ '
      else
        PS1='\[\e[38;2;137;180;250m\]\u@\h\[\e[0m\] \[\e[38;2;166;227;161m\]\w\[\e[0m\] \$ '
      fi
    ''
  );
}
