// Package domain defines shared values, VM identity, and operation state without I/O.
// Host path checks are lexical; filesystem resolution belongs to internal/filesystem.
// Callers must validate raw and persisted values; direct Go type conversions bypass constructors.
// Module identifiers validate syntax, not catalog membership.
//
// [Identity] records ownership and the exact host home allocation without configuration defaults.
// The state store enforces its immutability.
// [Status] describes a Limanix operation independently of the guest's power state.
// [Interrupted] is computed when listing abandoned operations and is never persisted.
package domain
