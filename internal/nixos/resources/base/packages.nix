{
  config,
  lib,
  pkgs,
  pinned,
  unstableSource,
  ...
}:
let
  runtime = builtins.fromJSON (builtins.readFile ./runtime.json);
  names = lib.unique runtime.packages;
  inherit (config.system.nixos) release;

  prefix = "unstable:";
  unstable = lib.hasPrefix prefix;
  attribute = lib.removePrefix prefix;

  # A listed name is an attribute path in the system's own package set, or with the prefix in the
  # catalog's nixos-unstable revision.
  resolve =
    name:
    let
      packages = if unstable name then pinned.${unstableSource.rev} else pkgs;
      source = if unstable name then "the catalog's nixos-unstable revision" else "nixpkgs ${release}";
      value = lib.attrByPath (lib.splitString "." (attribute name)) null packages;
    in
    if lib.isDerivation value then
      value
    else if builtins.isAttrs value then
      throw ''nixos.packages: "${name}" is a package set, not a package''
    else
      throw ''nixos.packages: "${name}" is not a package in ${source}'';

  listed = map (name: {
    inherit name;
    package = resolve name;
  }) names;

  # The directory whose bin/ holds a package's commands; NixOS also accepts packages as strings.
  commands = entry: if lib.isDerivation entry then toString (lib.getBin entry) else toString entry;

  listedDirectories = map (entry: commands entry.package) listed;
  others = lib.filter (
    entry: !(builtins.elem (commands entry) listedDirectories)
  ) config.environment.systemPackages;

  listedFile = pkgs.writeText "limanix-packages-listed" (
    lib.concatMapStrings (entry: "${entry.name}\t${commands entry.package}\n") listed
  );
  othersFile = pkgs.writeText "limanix-packages-others" (
    lib.concatMapStrings (entry: "${commands entry}\n") others
  );

  # The system profile ignores collisions, so a listed package would silently win or lose a
  # command against a module's or the platform's package. Every such conflict fails the build.
  check = pkgs.runCommand "limanix-packages-check" { } ''
    ${pkgs.runtimeShell} ${./packages-check.sh} ${listedFile} ${othersFile}
    touch "$out"
  '';
in
{
  # Naming a package allows its license. The names are constants, as nixpkgs requires: they come
  # from runtime.json, not from evaluating pkgs.
  nixpkgs.config.allowUnfreePackages = map (
    name: lib.last (lib.splitString "." (attribute name))
  ) names;
  lmx.pins = lib.optionalAttrs (lib.any unstable names) {
    ${unstableSource.rev} = unstableSource.sha256;
  };

  environment.systemPackages = map (entry: entry.package) listed;
  system.checks = lib.optional (names != [ ]) check;
}
