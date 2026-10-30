# Settings of [shell] and [toolchains] that the chosen catalog modules declare; a setting appears
# only when the configuration chooses its module.
{ lib, ... }:
let
  runtime = builtins.fromJSON (builtins.readFile ./runtime.json);
in
{
  config = lib.optionalAttrs (runtime.toolchains.rust != null) {
    lmx.rust.toolchain = runtime.toolchains.rust;
  };
}
