// Package nixos materializes embedded NixOS configuration and selected module snapshots.
//
// [Prepare] creates the host-side inputs for one VM generation. It does not run Nix, connect to a guest, or change VM state.
// internal/vm supplies the generation directory and holds module-source leases while copying; internal/guest later
// builds the inputs at /mnt/limanix/<generation> through the read-only generations mount.
// After a successful build, internal/vm switches the relative current selector and restarts the VM.
//
// The [catalog contract] defines the ownership, public interface, and compatibility requirements shared with the catalog.
//
// # Generation contents
//
//	<generation>/
//	├─ flake/
//	│  ├─ flake.nix + flake.lock + base NixOS modules
//	│  ├─ lmx.json                pinned lmx release for each guest system, or the local archives of cmd/bundle-lmx
//	│  ├─ interface.nix           catalog's public NixOS option declarations
//	│  ├─ platform.nix            catalog's guest platform: the base disk's layout and Lima integration
//	│  ├─ runtime.json            generation, user, architecture, ports, theme, shell, toolchains, module imports, packages
//	│  ├─ modules/lmx/_shared/    public declaration sources, including their private dependencies
//	│  ├─ modules/lmx/<kind>/<name>/  complete catalog entries, when the configuration chooses a standard module
//	│  └─ modules/<index>/        selected third-party module trees
//	├─ environment                systemd-compatible runtime assignments
//	└─ environment.sh             login-shell exports
//
// The catalog supplies the public NixOS interface, the guest platform and the pins of nixpkgs and nixos-lima with their locked
// graph. The client owns the flake template and its own base modules. Prepare combines these sources into a complete flake and lock file.
// [BaseImage] reads the first-boot disk and its digest from resources/image.json, the manifest of the bundled catalog release;
// the disk is the catalog's platform.nix written to a disk, and a rebuilt guest imports the same module to keep its layout and boot loader.
//
// Standard modules come from the limanix-modules release selected in Taskfile, not Go declarations.
// cmd/bundle-modules packages their trees, module.toml metadata, root interface.nix, platform.nix and flake.lock as resources/modules.zip
// before compilation, and publishes the release's image.json as resources/image.json.
// The executable reads these in memory; no catalog or manifest is downloaded at runtime.
//
//	Taskfile tag → limanix-modules archive + image.json → bundle-modules → embedded modules.zip + image.json
//	                                                            ↓ lmx:NAME[-VERSION]
//	                                                     VM generation snapshot
//
// The catalog keeps each module below its kind: shell components that [config.Config.Selection] chooses from [shell],
// toolchains from [toolchains], and modules that [nixos] modules selects by name. The module registry accepts only the
// last kind in [nixos] modules.
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
// Every generation includes lmx, the guest owner: its commands, its daemon, /etc/lmx/config.json and the help cards
// of the evaluated modules in /etc/lmx/help.json, rendered from the same declaration. The flake also exports them
// with those files as packages.<system>.lmx. The configuration's
// theme flavor sets lmx.capabilities.theme, which lmx and the catalog modules read; its shell prompt sets lmx.zsh.prompt and
// its Rust toolchain lmx.rust.toolchain, through resources/base/shell.nix. shell.dotfiles mounts at /mnt/dotfiles, adds the dotfiles key to config.json only
// then, since lmx 0.1 rejects it, and links the dotfiles in every interactive shell. The Bash fallback prompt
// uses the theme's colors only when no Starship provider is enabled; dumb terminals and NO_COLOR use plain text. Guest
// help, identity, and terminal compatibility remain available with an empty optional module selection.
//
// Read bundle.go for ordering, modules.go and resources.go for copying, runtime.go for the private flake JSON contract,
// environment.go for ENV encoding, resources/base/lmx.nix for lmx and its configuration,
// resources/base/packages.nix for nixos.packages and the check of their commands, and
// resources/base/workspace.nix for the fallback prompt.
//
// [catalog contract]: https://github.com/limanix/modules/blob/main/catalog/README.md
package nixos
