{
  config,
  lib,
  pkgs,
  ...
}:
let
  session = config.limanix.session;
  launcher = pkgs.writeShellScriptBin "limanix-session" ''
    limanix_session_command=${
      lib.escapeShellArg (if session.command == null then "" else toString session.command)
    }
    limanix_session_providers=${lib.escapeShellArg (lib.concatStringsSep ", " session.providers)}
    ${builtins.readFile ./session.sh}
  '';
in
{
  environment.systemPackages = [ launcher ];
}
