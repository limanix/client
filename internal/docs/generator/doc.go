// Package generator prepares model-derived references for Limanix documentation.
//
// [Generate] is the implementation behind cmd/docsgen. It renders configuration and command references from the
// same Go models and Cobra definitions used by the application. It does not start a VM, resolve application
// services, or render HTML.
//
// # Generated artifacts
//
//	config.Default + field tags → build/docs-generated/limanix.example.toml
//	                           └→ build/docs-generated/configuration.md
//	cli.Command                → build/docs-generated/cli.md
//	buildinfo.Version          → build/docs-generated/metadata.json
//
// These files are inputs for scripts/build_docs.py, which copies this repository's guides/ tree to build/docs/
// and places the generated files in build/docs/generated/. The separate documentation repository owns the theme
// and site publication.
//
// Generate renders all documents before writing them. Each file is replaced atomically, but the output set is
// not a multi-file transaction. Cancellation or an I/O error can leave a mixture of old and new files;
// rerunning Generate refreshes the set.
//
// The repository's docs/prepare task runs this generator before the Python assembly script.
// This package does not embed website output into the Limanix runtime binary.
//
// Read render.go for the artifact list and source models, and generate.go for directory preparation, cancellation,
// writes, and diagnostics.
package generator
