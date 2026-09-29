// Package catalog reads the standard NixOS module catalog packaged with Limanix.
//
// A catalog is one ZIP containing its release tag, upstream LICENSE, flake.lock, and module trees. Its ZIP comment identifies [Repository];
// archives from another source are rejected. Module names come from modules/<name>; descriptions come from module.toml.
// Optional default and versions fields expose NAME-VERSION selectors. Their entry points are versions/<version>.nix inside
// the same module tree. NAME always selects default.nix; the module itself implements its default. Duplicate selectors are rejected.
// No module identifiers or Nix implementations are declared in Go.
//
// [Open] validates archive paths, file types, contents, the nixpkgs pin, and metadata before exposing read-only module filesystems.
// [Catalog.Nixpkgs] returns the catalog's declared GitHub input URL and self-contained locked node for the guest flake.
// Catalogs without a supported pin are rejected. The catalog reader neither
// evaluates Nix nor accesses the network. The repository marker and release tag identify the downloaded source. These metadata
// and ZIP integrity checks do not prove the authenticity of an upstream release.
package catalog
