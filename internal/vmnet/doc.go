// Package vmnet installs the macOS network helper used by Lima's QEMU backend.
//
// [Setup.Ensure] checks the installation as the regular user.
// A valid setup needs no prompt or writes.
// Missing setup requires interactive confirmation followed by sudo authentication.
// Noninteractive calls return the setup error.
//
// [Install] accepts a configuration snapshot and restricts writes to protected host paths.
// Helper bytes come from the executable's embedded bundle.
// Sudoers rules come from Lima, are checked by visudo, and replace the file atomically after a backup.
// The configured group is preserved.
//
// Lima owns network configuration, daemon lifetime, socket paths, and command authorization.
// This package does not rewrite networks.yaml or install QEMU.
package vmnet
