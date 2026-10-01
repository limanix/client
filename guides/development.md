# Development

The client repository owns the CLI, VM lifecycle, configuration model, and macOS binaries.
The module catalog lives in `limanix/modules`; the documentation site lives in `limanix/docs`.
Use `Taskfile.yml` for checks, bundled resources, and native builds.

## Prepare your tools

| Work                                                                     | Requirements                                                                                         |
|--------------------------------------------------------------------------|------------------------------------------------------------------------------------------------------|
| Formatting, lint, tests, vulnerability checks, documentation preparation | Task 3.53.1 or newer, Docker with a running engine, network access for pinned tools and dependencies |
| Native client build                                                      | macOS, the Go version in `go.mod`, Task, Xcode command-line tools                                    |
| Run the client                                                           | macOS 26 or newer, a binary matching the Mac's architecture                                          |
| Preview documentation                                                    | Sibling `client`, `modules`, and `docs` checkouts, Task, and Docker                                  |

Clone the client and list its tasks:

```console
git clone https://github.com/limanix/client.git
cd client
task --list
```

The examples use `task --yes` to accept the pinned remote Taskfile includes.

## Source layout

| Path                                          | Responsibility                                                                |
|-----------------------------------------------|-------------------------------------------------------------------------------|
| `cmd/limanix/`, `internal/cli/`               | CLI entry point, commands, flags, output                                      |
| `internal/config/`, `internal/domain/`        | TOML parsing, defaults, validation, domain types                              |
| `internal/vm/`                                | VM lifecycle and configuration generations                                    |
| `internal/lima/`, `internal/hostagent/`       | Lima integration and host-side VM process                                     |
| `internal/state/`, `internal/managedhome/`    | Saved records and managed home ownership                                      |
| `internal/modules/`                           | Local imports and module selection                                            |
| `internal/nixos/`                             | Guest configuration and bundled catalog                                       |
| `internal/bundle/`                            | Embedded guest agents and network helper                                      |
| `cmd/build-docs/`, `internal/docs/generator/` | Documentation preparation and reference generation                            |
| `guides/`                                     | Handwritten documentation                                                     |
| `.github/workflows/`                          | CI checks, release version preparation, publication, and documentation events |

## Run the checks

The shared Go tasks run in containers.
For tests, choose a published module catalog tag and pass it as `modules_version`; replace `v4` below with that tag.

```console
task --yes ci/golang-fmt
task --yes ci/golang-lint
task --yes ci/golang-test modules_version=v4
task --yes ci/golang-vuln
task --yes ci/nixos-eval modules_version=v4
```

| Task             | What it checks                                                            |
|------------------|---------------------------------------------------------------------------|
| `ci/golang-fmt`  | Go formatting under `cmd/` and `internal/`, without rewriting files       |
| `ci/golang-lint` | Go source and tests                                                       |
| `ci/golang-test` | Go tests with the race detector, after preparing embedded resources       |
| `ci/golang-vuln` | Known vulnerabilities in the client and its embedded Lima guest agent     |
| `ci/nixos-eval`  | Generated guest flake evaluation for the selected client and catalog pair |

Formatting, linting, and vulnerability checks do not require a published module catalog.
Tests and native builds prepare the embedded resources before running.

`ci/nixos-eval` runs independently of the Go suite for AMD64 and ARM64, with an empty module selection, every catalog selector individually, and supported integration cases.
The integration test calls the same `Prepare` function used for VM generations and evaluates the generated flake's system derivation in the Nix container with its lock file unchanged.
It does not build packages or boot a VM.
The ordinary Go test suite does not require Nix.
The release workflow runs the same evaluation against the exact client checkout and catalog tag in parallel with native builds and documentation preparation.
Publication waits for all three jobs to succeed.
A client revision without the pair-validation task cannot publish an unverified pair.

The PR workflow runs shared Go checks and generated guest flake evaluation in separate jobs after selecting the catalog tag.
The native macOS build follows the Go checks; `gate` combines the catalog selection, Go, evaluation, and build results.
A separate workflow checks the PR label.

For lifecycle or guest configuration changes, also exercise the operation with a disposable VM on macOS.
Use a separate configuration and [`LIMANIX_HOME`](troubleshooting.md#state-directories) to keep its state separate from your working VMs.
Tests and a signed build do not establish that the affected VM operation succeeds.

## Build a native client

Run on your Mac with the module catalog tag you want to bundle:

```console
task --yes ci/build modules_version=v4
```

This task runs natively without Docker.
`modules_version` is required; Task does not select or look up a catalog version.
By default, it prepares embedded resources, builds both architectures, applies an ad-hoc signature with the virtualization entitlement, and verifies the signature and macOS deployment target.
The outputs are `bin/limanix-arm64` for Apple Silicon and `bin/limanix-amd64` for Intel.

To build only one architecture, pass `TARGET_ARCH=arm64` or `TARGET_ARCH=amd64`:

```console
task --yes ci/build modules_version=v4 TARGET_ARCH=arm64
```

CI builds both architectures in parallel, each on a matching macOS runner.

To install the Apple Silicon build:

```console
mkdir -p ~/.local/bin
install -m 755 bin/limanix-arm64 ~/.local/bin/limanix
export PATH="$HOME/.local/bin:$PATH"
limanix --version
```

Use `bin/limanix-amd64` on Intel and keep `~/.local/bin` in your shell's `PATH`.
A plain `go build` does not perform the resource preparation and signing sequence.

Set the embedded client version with `RELEASE_TAG`:

```console
task --yes ci/build modules_version=v4 RELEASE_TAG=v1.2.3+1
```

This builds local files; it does not create a Git tag or publish a release.

### Embedded resources

| Resource                      | Source of truth                                             | Preparation                                                    |
|-------------------------------|-------------------------------------------------------------|----------------------------------------------------------------|
| NixOS catalog and Nixpkgs pin | Explicit `modules_version` task argument                    | `cmd/bundle-modules` downloads the selected modules tag        |
| Linux guest agents            | Lima dependency in `go.mod`                                 | `cmd/bundle-guestagent` builds `amd64` and `arm64` agents      |
| macOS network helper          | `socket_vmnet` version, hashes, and sizes in `Taskfile.yml` | `cmd/bundle-socketvmnet` downloads and validates both archives |

Generated archives are ignored by Git.
Tasks that prepare the catalog require the selected modules tag to exist upstream.

The catalog's `flake.lock` supplies the Nixpkgs revision for catalog checks and guest builds.
The client rejects catalogs without that pin or their root `interface.nix`.
Public `catalog/_shared/*.nix` declarations are loaded in every guest configuration; private `_shared/internal/` files are imported only by their consumers.
Follow [Update the NixOS base](https://limanix.dev/categories/nixos/writing-modules.html#update-the-nixos-base) to change it.

The client owns `internal/nixos/resources/flake.nix.tmpl` and `flake.lock.tmpl`, including the `nixos-lima` dependency graph.
The catalog supplies the `nixpkgs` input when the client prepares a VM generation.
Guest rebuilds reject inputs that would require a lock update.
The bootstrap image, its checksums in `internal/nixos/image.go`, and `system.stateVersion` are also maintained in the client.

## Maintain the documentation

Edit explanations and examples in `guides/`.
The TOML blocks in `getting-started.md` and `configuration.md` mirror the downloadable files in `guides/examples/`; update both copies together.
Then prepare the pages:

```console
task --yes docs/prepare
```

The Go command `cmd/build-docs` copies `guides/` into a clean `build/docs/` directory and generates references under `build/docs/generated/` from the local command definitions and configuration model.
This task does not download the module catalog or require a published release.
The output directory is ignored by Git.

| File under `build/docs/generated/` | Contents                                       |
|------------------------------------|------------------------------------------------|
| `cli.md`                           | Commands, flags, and help text                 |
| `configuration.md`                 | Fields and their descriptions                  |
| `limanix.example.toml`             | Example derived from model defaults            |
| `metadata.json`                    | Client version used by the documentation build |

Update source definitions instead of editing generated files.
To set the version recorded in the generated metadata:

```console
task --yes docs/prepare RELEASE_TAG=v1.2.3+4
```

Replace the example version with the version being documented.
The task prepares Markdown and supporting files; the [docs repository](https://github.com/limanix/docs) owns HTML builds and publication.
Client releases package these files as `docs.tar.gz` alongside the binaries.
The docs release workflow downloads this archive and the matching modules archive to build the site.

For a local preview, run from the sibling `docs` repository:

```console
task --yes docs/serve CLIENT_ROOT=../client MODULES_ROOT=../modules
```

Open `http://127.0.0.1:8040`.
The preview uses both working trees without requiring published tags and rebuilds when their sources change.

## Contribute a change

Update the implementation, relevant tests, and user guide together.
Run the checks for the changed behavior and describe any VM scenario you tested in the PR.
Review the diff for generated archives, private configuration, and unrelated changes before submitting it.
