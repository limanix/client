package domain

// VMName identifies a VM within the host's Limanix state.
type VMName string

// Username identifies the unprivileged development user in the guest.
type Username string

// GuestPath is a normalized absolute path inside the guest.
type GuestPath string

// ModuleName identifies a NixOS module within its catalog.
type ModuleName string

// ModuleID includes the source namespace of a selected module.
type ModuleID string

// PackageName is an attribute path in the guest's nixpkgs, such as jq or python3Packages.ipython, or in the catalog's
// nixos-unstable revision with the unstable: prefix, such as unstable:codex.
type PackageName string

// ToolchainLine is a version line of a catalog toolchain, such as 1.27; empty chooses none.
type ToolchainLine string

// RustToolchain is a rustup toolchain: a channel, a channel with an archive date, or a Rust version; empty chooses
// none.
type RustToolchain string

// OptionalHostPath is a host path that may be empty to choose none.
type OptionalHostPath string

// EnvName is a POSIX environment variable name.
type EnvName string

// EnvValue is a literal guest environment value.
type EnvValue string
