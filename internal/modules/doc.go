// Package modules imports trusted NixOS module trees and resolves selected sources.
//
// Standard modules use lmx:NAME or a catalog-declared lmx:NAME-VERSION selector.
// Imported modules are copied beneath the state root and selected as third-party:NAME.
// [Registry] receives catalog metadata from internal/nixos and does not evaluate Nix code.
//
// Imports are copied before publication under an exclusive lock.
// Removal detaches an import under that lock, then deletes its files after releasing it.
// A [SourceSet] holds a shared lock until its caller finishes copying all selected sources.
// Imported paths must not be used after [SourceSet.Close].
// Existing VM generations retain their independent snapshots.
//
// Copying rejects symlinks, special files, existing destinations, and destinations inside the source.
// These checks do not sandbox or audit the Nix code.
// Registry locks coordinate Limanix operations, not unrelated processes editing the same files.
package modules
