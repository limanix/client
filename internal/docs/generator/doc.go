// Package generator prepares the Limanix documentation tree.
//
// [Generate] is the implementation behind cmd/build-docs. It copies guides/ and renders configuration and
// command references from the same Go models and Cobra definitions used by the application. It does not start
// a VM, resolve application services, or render HTML.
//
// # Generated artifacts
//
//	guides/                    → build/docs/
//	config.Default + field tags → build/docs/generated/limanix.example.toml
//	                           └→ build/docs/generated/configuration.md
//	cli.Command                → build/docs/generated/cli.md
//	buildinfo.Version          → build/docs/generated/metadata.json
//
// The prepared tree is ready for the release documentation archive. The separate documentation repository
// owns the theme and site publication.
//
// Generate renders the references before replacing build/docs and rejects symlinks at build/ or build/docs/.
// Generated files are written atomically, but the tree is not a transaction: cancellation or an I/O error can
// leave an incomplete output. Rerunning Generate refreshes the complete tree and removes stale files.
//
// The repository's docs/prepare task runs this generator.
// This package does not embed website output into the Limanix runtime binary.
//
// Read render.go for the artifact list and source models, and generate.go for directory preparation, cancellation,
// writes, and diagnostics.
package generator
