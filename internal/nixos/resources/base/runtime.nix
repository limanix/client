{
  config,
  lib,
  pkgs,
  runtime,
  ...
}:
let
  environmentDropIn = ''
    [Service]
    EnvironmentFile=-/etc/limanix/environment
  '';
in
{
  options.limanix.user = {
    name = lib.mkOption {
      type = lib.types.str;
      readOnly = true;
      description = "Development account name supplied by Limanix.";
    };
    home = lib.mkOption {
      type = lib.types.str;
      readOnly = true;
      description = "Development account home supplied by Limanix.";
    };
    shell = lib.mkOption {
      type = lib.types.package;
      default = pkgs.bashInteractive;
      defaultText = lib.literalExpression "pkgs.bashInteractive";
      description = "Login shell for the development account.";
    };
  };

  config = {
    limanix.user = {
      inherit (runtime.user) name home;
    };

    networking.hostName = runtime.name;
    networking.firewall.allowedTCPPorts = runtime.ports.tcp;
    networking.firewall.allowedUDPPorts = runtime.ports.udp;

    systemd.services.lima-init = {
      unitConfig.DefaultDependencies = false;
      requires = [ "sysinit.target" ];
      after = lib.mkForce [ "sysinit.target" ];
      before = [
        "basic.target"
        "shutdown.target"
      ];
      conflicts = [ "shutdown.target" ];
      requiredBy = [ "basic.target" ];
      postStart = ''
        set -o pipefail
        directory="$(${pkgs.coreutils}/bin/mktemp -d /run/limanix-mounts.XXXXXX)"
        trap '${pkgs.coreutils}/bin/rm -rf -- "$directory"' EXIT

        ${pkgs.gawk}/bin/awk '
          /^#LIMA-START$/ { lima = 1; next }
          /^#LIMA-END$/ { lima = 0 }
          lima
        ' /etc/fstab > "$directory/fstab"
        ${pkgs.util-linux}/bin/findmnt --fstab --tab-file "$directory/fstab" \
          --json --output TARGET \
          | ${pkgs.jq}/bin/jq -j '.filesystems[].target + "\u0000"' \
          > "$directory/targets"
        while IFS= read -r -d "" target; do
          unit="$(${pkgs.systemd}/bin/systemd-escape --path --suffix=mount "$target")"
          ${pkgs.systemd}/bin/systemctl start -- "$unit"
        done < "$directory/targets"
      '';
    };

    users.groups.${runtime.user.name} = { };
    users.users.${runtime.user.name} = {
      isNormalUser = runtime.user.uid >= 1000;
      isSystemUser = runtime.user.uid < 1000;
      uid = runtime.user.uid;
      group = runtime.user.name;
      home = runtime.user.home;
      shell = config.limanix.user.shell;
      createHome = false;
      linger = true;
    };
    security.sudo.wheelNeedsPassword = true;
    security.sudo.extraRules = [
      {
        users = [ "limanix-admin" ] ++ lib.optional runtime.user.sudo runtime.user.name;
        commands = [
          {
            command = "ALL";
            options = [ "NOPASSWD" ];
          }
        ];
      }
    ];

    environment.extraInit = ''
      if [ "$(${pkgs.coreutils}/bin/id -u)" = "${toString runtime.user.uid}" ]; then
        export XDG_RUNTIME_DIR="/run/user/${toString runtime.user.uid}"
        export DBUS_SESSION_BUS_ADDRESS="unix:path=$XDG_RUNTIME_DIR/bus"
      fi
      if [ -r /etc/limanix/environment.sh ]; then
        . /etc/limanix/environment.sh
      fi
    '';
    systemd.packages =
      builtins.map
        (
          scope:
          pkgs.writeTextDir "lib/systemd/${scope}/service.d/10-limanix-environment.conf" environmentDropIn
        )
        [
          "system"
          "user"
        ];
  };
}
