// Package nixos materializes embedded NixOS configuration and selected module snapshots.
//
// [Prepare] creates the host-side inputs for one VM generation. It does not run Nix, connect to a guest, or change VM state.
// internal/vm supplies the generation directory and holds module-source leases while copying; internal/guest later
// applies the result through the read-only /mnt/limanix mount.
//
// The [catalog contract] defines the ownership, public interface, and compatibility requirements shared with the catalog.
//
// # Generation contents
//
//	<generation>/
//	├─ flake/
//	│  ├─ flake.nix + flake.lock + base NixOS modules
//	│  ├─ lmx.json                pinned lmx release for each guest system
//	│  ├─ interface.nix           catalog's public NixOS option declarations
//	│  ├─ runtime.json            generation, user, architecture, ports, module imports
//	│  ├─ modules/lmx/_shared/    public declaration sources, including their private dependencies
//	│  ├─ modules/lmx/<name>/     complete catalog entries, when a standard module is selected
//	│  └─ modules/<index>/        selected third-party module trees
//	├─ environment                systemd-compatible runtime assignments
//	└─ environment.sh             login-shell exports
//
// The catalog supplies the public NixOS interface and the guest's nixpkgs pin. The client owns the flake templates, platform modules and locked
// nixos-lima dependency graph. Prepare combines these embedded sources into a complete flake and lock file.
// [BaseImage] derives the first-boot disk URL from the platform lock template. Image digests live in image.go;
// changing that dependency requires reviewing the partition and boot configuration in resources/base/platform.nix.
//
// Standard modules come from the limanix-modules release selected in Taskfile, not Go declarations.
// cmd/bundle-modules packages their trees, module.toml metadata, root interface.nix and flake.lock as resources/modules.zip before compilation.
// The executable reads this archive in memory; no catalog is downloaded or installed at runtime.
//
//	Taskfile tag → limanix-modules archive → bundle-modules → embedded modules.zip
//	                                                            ↓ lmx:NAME[-VERSION]
//	                                                     VM generation snapshot
//
// Standard modules share one catalog snapshot; sibling Nix imports retain their paths. Root _shared/*.nix schemas and infrastructure
// enter runtime.json before selected entry points, including with no standard modules selected; _shared/test.nix is excluded.
// Other shared files remain in the snapshot and are not loaded automatically.
// Third-party modules remain separate snapshots.
// Explicit versions select versions/<version>.nix from their module directory; unversioned names select default.nix.
// [SystemModules] returns an independent metadata map for the registry. An empty selection adds no optional modules;
// the catalog's interface, public shared declarations, nixpkgs pin and the client's base still apply.
//
// # Runtime environment and failure contract
//
// ENV values are written literally with target-specific escaping. They remain beside the flake and are absent
// from runtime.json and flake source inputs. They are plaintext runtime configuration, not encrypted secret storage;
// guest installation makes them guest-wide environment settings.
//
// Prepare requires a fresh flake destination and a positive host UID. It may leave partial output on failure;
// the VM generation owner decides whether to discard it or retain it for recovery. Individual writes do not make
// an entire generation transactional.
//
// Every generation includes lmx, the guest owner: its commands, its daemon and /etc/lmx/config.json, rendered from
// the same declaration. The flake also exports them with that file as packages.<system>.lmx. The Bash fallback prompt
// uses the theme's colors only when no Starship provider is enabled; dumb terminals and NO_COLOR use plain text. Guest
// help, identity, and terminal compatibility remain available with an empty optional module selection.
//
// Read bundle.go for ordering, modules.go and resources.go for copying, runtime.go for the private flake JSON contract,
// environment.go for ENV encoding, resources/base/lmx.nix for lmx and its configuration, and
// resources/base/workspace.nix for the fallback prompt.
//
// [catalog contract]: https://limanix.dev/categories/nixos/catalog-contract.html
package nixos
