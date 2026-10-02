{
  lib,
  modulesPath,
  pkgs,
  ...
}:
{
  imports = [
    (modulesPath + "/profiles/qemu-guest.nix")
    (
      { lib, ... }:
      {
        # Profile inputs stay stable when selected module imports repeat.
        options.environment.systemPackages = lib.mkOption {
          apply =
            packages:
            let
              priority =
                package:
                if builtins.isAttrs package then
                  package.meta.priority or lib.meta.defaultPriority
                else
                  lib.meta.defaultPriority;
            in
            lib.sort (
              left: right:
              if toString left == toString right then
                priority left < priority right
              else
                toString left < toString right
            ) packages;
        };
      }
    )
  ];

  environment.systemPackages = [ pkgs.ghostty.terminfo ];

  services.lima.enable = true;
  users.mutableUsers = true;
  services.openssh.enable = true;
  nix.settings.experimental-features = [
    "nix-command"
    "flakes"
  ];

  boot.growPartition = true;
  boot.loader.grub = {
    device = "nodev";
    efiSupport = true;
    efiInstallAsRemovable = true;
  };
  fileSystems."/boot" = {
    device = lib.mkForce "/dev/vda1";
    fsType = "vfat";
  };
  fileSystems."/" = {
    device = "/dev/disk/by-label/nixos";
    autoResize = true;
    fsType = "ext4";
    options = [
      "noatime"
      "nodiratime"
      "discard"
    ];
  };

  system.stateVersion = "26.05";
}
