{ pkgs, ... }:
{
  environment.systemPackages = [ pkgs.hello ];
  limanix.help.fixture = {
    summary = "Synthetic catalog module.";
    commands = [ "hello" ];
  };
}
