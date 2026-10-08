{
  config,
  lib,
  modulesPath,
  pkgs,
  ...
}:
let
  inherit ((builtins.fromJSON (builtins.readFile ./runtime.json))) disk;
in
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

  # lmxd collects unreferenced paths after each update and whenever free space or inodes fall below
  # the collect limit. ext4 sizes its inode table with the disk, and a Nixpkgs source tree alone
  # uses ~90k.
  nix.settings.auto-optimise-store = lib.mkDefault true;

  # Builds collect garbage themselves when free bytes fall below the minimum. lmxd checks both
  # limits between builds, because Nix has no inode equivalent.
  nix.settings.min-free = lib.mkDefault (disk.bytes * disk.minimumPercent / 100);
  nix.settings.max-free = lib.mkDefault (disk.bytes * disk.collectPercent / 100);

  # The journal is persistent. This keeps weeks of logs, including the previous boot that
  # `lmx logs --previous` reads, without letting them fill a small guest disk.
  services.journald.extraConfig = "SystemMaxUse=512M";

  # Configurations come from the generated flake. Disabling channels keeps their state files,
  # and without the rules below the base image's copy of Nixpkgs would stay a garbage-collector
  # root.
  nix.channel.enable = lib.mkDefault false;
  systemd.tmpfiles.rules = lib.mkIf (!config.nix.channel.enable) [
    "r /nix/var/nix/profiles/per-user/root/channels"
    "r /nix/var/nix/profiles/per-user/root/channels-*-link"
    "r /root/.nix-defexpr/channels"
    "r /root/.nix-channels"
  ];

  # Doc outputs, such as the Rust standard library HTML, cost many inodes in a headless guest.
  documentation.doc.enable = lib.mkDefault false;

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
