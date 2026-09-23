// Package generator prepares model-derived references for Limanix documentation.
//
// [Generate] is the implementation behind cmd/docsgen. It renders configuration and command references from the
// same Go models and Cobra definitions used by the application. It does not start a VM, resolve application
// services, or render HTML.
//
// # Generated artifacts
//
//	config.Default + field tags → docs/_generated/limanix.example.toml
//	                           └→ docs/_generated/configuration.md
//	cli.Command                → docs/_generated/cli.md
//	buildinfo.Version          → docs/_generated/metadata.json
//
// These files are inputs for a separate documentation build. This package owns their content;
// the documentation repository owns the handwritten guides, theme, and site publication.
//
// Generate renders all documents before writing them. Each file is replaced atomically, but the output set is
// not a multi-file transaction. Cancellation or an I/O error can leave a mixture of old and new files;
// rerunning Generate refreshes the set.
//
// The repository's docs/generate task runs this generator.
// This package does not embed website output into the Limanix runtime binary.
//
// Read render.go for the artifact list and source models, and generate.go for directory preparation, cancellation,
// writes, and diagnostics.
package generator
