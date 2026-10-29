// Package nixos prepares the host files for a VM generation.
//
// [Prepare] combines the embedded catalog, selected third-party modules, and VM configuration.
// It does not run Nix or change VM state.
// The VM manager owns the generation directory and holds module-source leases while files are copied.
// The guest builds these inputs through the read-only generations mount.
//
// The catalog supplies the public interface, guest platform, and locked nixpkgs and nixos-lima inputs.
// The client supplies the flake template, runtime settings, and base modules.
// Catalog files and the base-image manifest are embedded at build time.
// No catalog download is needed at runtime.
//
// Standard modules share one snapshot, preserving imports between modules.
// Direct _shared/*.nix declarations are loaded before selected modules; _shared/test.nix is excluded.
// Third-party modules use separate snapshots.
// An empty module selection retains the platform, shared declarations, and guest owner lmx.
//
// Environment values are escaped for systemd and shell syntax and written beside the flake.
// They are plaintext guest-wide settings, excluded from runtime.json and Nix source inputs.
// Prepare requires a new flake directory and may leave partial output on failure.
// The VM manager decides whether to retain that output for recovery.
package nixos
