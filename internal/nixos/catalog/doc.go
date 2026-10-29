// Package catalog reads the NixOS module catalog embedded in Limanix.
//
// [Open] validates ZIP paths, file types, contents, metadata, and locked Nix inputs.
// The archive must contain its release tag, LICENSE, interface.nix, platform.nix, flake.lock, and modules.
// The ZIP comment must match [Repository].
// These checks verify structure and integrity, not release authenticity.
//
// Modules live under modules/<kind>/<name> and declare their metadata in module.toml.
// Unversioned selectors use default.nix; versioned selectors use versions/<version>.nix.
// Version lines must be unique, and a nonempty list requires a default from that list.
// Duplicate selectors are rejected across all kinds.
// Module implementations and version lists belong to the catalog.
//
// The _shared tree remains available to module imports without exposing a selector.
// Its direct .nix files are public declarations, except test.nix.
// [Catalog] returns copies of metadata and lock data and a read-only source filesystem.
// This package does not evaluate Nix or access the network.
package catalog
