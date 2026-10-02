// Package modulegen prepares an embedded catalog from a limanix/modules tag or a local checkout.
//
// Generate reads the GitHub source archive over HTTPS and maps catalog/ source trees to modules/ in the embedded archive.
// It preserves public and private _shared declarations in the same tree, retains LICENSE, root interface.nix
// and flake.lock, and records the source repository and selected release tag.
// The catalog package validates the result before
// one atomic file replacement publishes resources/modules.zip.
//
// The published archive doubles as the build cache. A valid archive from the expected
// repository with the requested tag, public interface and a supported NixOS pin is reused without a network request.
// Old archives without interface.nix or flake.lock are rebuilt. Tags are not checksum pins; moving a tag does
// not invalidate an existing cache. No Nix evaluation or runtime module installation occurs here.
// Local checkout builds use [GenerateLocal]. Each invocation validates and packages current source;
// only identical output bytes are reused.
package modulegen
