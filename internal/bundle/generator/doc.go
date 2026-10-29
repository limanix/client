// Package generator builds the Linux guest agents embedded by internal/bundle.
//
// [Generate] resolves the pinned Lima module and Go compiler, then checks existing assets.
// Reuse requires matching source checksums, build settings, manifest entries, and executable metadata.
// A failed reuse check is logged and triggers a rebuild.
//
// Lima must be a versioned module without a replacement.
// Dependency checks use the guest-agent package graph.
// Build commands use the resolved compiler with GOTOOLCHAIN=local.
// Both arm64 and amd64 binaries must have the expected ELF architecture and no dynamic interpreter.
//
// All targets are built and validated before publication.
// Archives are replaced atomically and the manifest is written last.
// Publication is not a transaction across files.
// Run generation before compiling the embedding application.
package generator
