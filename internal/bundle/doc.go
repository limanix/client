// Package bundle supplies embedded Linux guest agents and the macOS network helper.
//
// [Cache] exposes validated guest-agent archives as private host files for Lima.
// Embedded gzip contents and ELF architecture are checked once per architecture in each process.
// Every [Cache.Path] call verifies the cached bytes and 0600 permissions.
// Missing archives are written atomically; mismatched archives return an error.
// The cache directory is named by the compressed payload's SHA-256 digest.
//
// [SocketVMNet] verifies the pinned archive, Mach-O architecture, and deployment target.
// It returns executable and license bytes to package vmnet for privileged installation.
// Taskfile supplies the helper version, archive sizes, and digests through linker flags.
// Build-time generation belongs to the generator and vmnetgen subpackages.
package bundle
