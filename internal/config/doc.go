// Package config defines the TOML configuration, defaults, and generated reference.
// Field tags keep parsing, validation, the example file, and the fields of [Reference] consistent.
// The configuration schema is versioned separately from releases and state records.
//
// [Parse] validates literal input without filesystem access.
// [Load] resolves relative host paths from the real configuration file's directory.
// Mount sources need not exist until VM preflight.
// Environment values remain literal strings.
// Module availability is checked by the module registry, after configuration validation.
//
// [Error] identifies a field without exposing its value.
// Its underlying cause may contain input and must not replace the public diagnostic.
package config
