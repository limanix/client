# Develop the client

The client repository owns the CLI, VM lifecycle, configuration model, and
macOS binaries. The module catalog lives in `limanix/modules`; the Sphinx site
lives in `limanix/docs`.

Use the root `Taskfile.yml` as the entry point for checks, asset generation, and
native builds. Its tasks supply the versions and build flags expected by the
client.

## Prepare your tools

| Work | Requirements |
| --- | --- |
| Formatting, lint, tests, vulnerability checks, generated references | Task 3.53.1 or newer, Docker with a running engine, network access for pinned tooling and dependencies |
| Native client build | macOS, the Go version declared in `go.mod`, Task, Xcode command-line tools |
| Try the built client | macOS 26 or newer, a binary matching the Mac's architecture |
| Preview the Sphinx site | A sibling `limanix/docs` checkout, Task, and Docker |

The shared Go tasks run tools in containers. `ci/build` runs natively and does
not require Docker. It uses CGO and Apple's tools to build and sign both macOS
architectures.

Clone the client and enter its directory:

```console
git clone https://github.com/limanix/client.git
cd client
task --list
```

The examples use `task --yes` to accept the pinned remote Taskfile include.

## Start with the part you are changing

| Path | Responsibility |
| --- | --- |
| `cmd/limanix/`, `internal/cli/` | CLI entry point, commands, flags, output |
| `internal/config/`, `internal/domain/` | TOML parsing, defaults, validation, domain types |
| `internal/vm/` | VM creation, updates, lifecycle, generation handling |
| `internal/lima/`, `internal/hostagent/` | Lima integration and host-side VM process |
| `internal/state/`, `internal/managedhome/` | Saved VM records and managed home ownership |
| `internal/modules/` | Local imports and module selection |
| `internal/nixos/` | Guest configuration, bundled catalog, generated NixOS sources |
| `internal/bundle/` | Embedded guest agents and network helper |
| `internal/docs/generator/` | CLI and configuration reference generation |
| `guides/` | Handwritten user guides and examples |

Read the existing tests near the behavior you are changing. In particular,
resource ownership, failed updates, and deletion have behavior beyond the CLI
flags; trace the manager and state code before changing them.

## Run the checks

From the client repository:

```console
task --yes ci/fmt
task --yes ci/lint
task --yes ci/test
task --yes ci/vuln
```

| Task | What it checks |
| --- | --- |
| `ci/fmt` | Formatting under `cmd/` and `internal/`; reports files without rewriting them |
| `ci/lint` | Go source and tests with the shared linter |
| `ci/test` | Go tests with the race detector, after preparing embedded assets |
| `ci/vuln` | Known vulnerabilities in the client and the embedded Lima guest agent |

For changes to release automation, also run:

```console
python3 -m unittest discover -s .github/scripts -p 'test_*.py'
```

The PR workflow checks these release scripts, a PR label, the shared Go checks,
and a native macOS build. Its final `gate` combines those results. Add an
appropriate label to your PR; an unlabeled PR fails the label check.

## Build a native client

On your **Mac**:

```console
task --yes ci/build
```

The outputs are `bin/limanix-arm64` and `bin/limanix-amd64`. The task:

1. Downloads or prepares the pinned embedded resources.
2. Builds both architectures with the configured macOS deployment target.
3. Applies an ad-hoc signature with the VM entitlement.
4. Verifies the signature and deployment target.

Use `bin/limanix-arm64` on Apple Silicon and `bin/limanix-amd64` on Intel. A plain
`go build` does not perform this preparation and signing sequence. Use the Taskfile
build when testing VM behavior.

For a versioned build, `RELEASE_TAG` sets the version embedded in the binary:

```console
task --yes ci/build RELEASE_TAG=v1.2.3+1
```

This command builds local files. It does not create a Git tag or publish a release.
The central documentation describes the release process.

### Where the embedded files come from

| Asset | Source of truth | Preparation |
| --- | --- | --- |
| NixOS catalog and guest Nixpkgs pin | `modules_version` in `Taskfile.yml`, selecting a `limanix/modules` tag | `cmd/bundle-modules` |
| Linux Lima guest agents | Lima dependency in `go.mod` | `cmd/bundle-guestagent` builds `amd64` and `arm64` agents |
| macOS network helper | `socket_vmnet` version, hashes, and sizes in `Taskfile.yml` | `cmd/bundle-socketvmnet` downloads and validates both archives |

The generated archives are ignored by Git. Keep the pins and generators in source
control, not generated binary bundles.

The catalog's root `flake.lock` supplies the Nixpkgs revision for both catalog checks and guest builds.
The client requires that pin and rejects catalogs that omit it.
To update it, follow [Update the NixOS base](https://limanix.dev/categories/nixos/writing-modules.html#update-the-nixos-base).

The client owns `internal/nixos/resources/flake.nix.tmpl` and `flake.lock.tmpl`.
These templates keep the `nixos-lima` dependency graph; the catalog supplies the missing `nixpkgs` input when the client prepares a VM generation.
The generated `flake.nix` and `flake.lock` must agree: guest rebuilds reject any input that would require a lock update.
The bootstrap image, its checksums in `internal/nixos/image.go`, and `system.stateVersion` remain client-owned.

```{important}
A clean checkout needs the modules tag selected by `modules_version` to
exist upstream. If that tag has not been published, tasks that bundle the catalog
cannot complete. Check the configured tag and the upstream repository when a
download fails; do not substitute a different catalog silently.
```

Tests and a signed build do not prove that a VM starts successfully. For a
lifecycle or guest configuration change, also exercise the affected operation
with a disposable VM on macOS. Use a separate configuration and
[`LIMANIX_HOME`](storage-and-recovery.md) to keep that test's state separate from
your working VMs.

## Maintain the documentation

Keep explanations and examples in `guides/`.
Prepare the documentation from the client repository:

```console
task --yes docs/prepare
```

The task prepares the configured module catalog, then runs the Go generator to derive references from the runtime command definitions and configuration model.
The generator writes intermediate files to `build/docs-generated/`.
The Python script `scripts/build_docs.py` copies `guides/` and those references into a clean `build/docs/` tree.
Generated references are stored under `build/docs/_generated/`:

| File | Contents |
| --- | --- |
| `cli.md` | Commands, flags, and help text |
| `configuration.md` | Configuration fields and their descriptions |
| `limanix.example.toml` | Example derived from configuration defaults |
| `metadata.json` | Client version used by the documentation build |

**Do not edit generated files.**
Update the guides, command definitions, or configuration models, then rerun the task.
Both output directories are ignored by Git.

To set the client version in the generated metadata, pass `RELEASE_TAG`:

```console
task --yes docs/prepare RELEASE_TAG=v1.2.3+4
```

`v1.2.3+4` is an example client version; replace it with the version being documented.
The task prepares Markdown and supporting files; it does not build HTML, create a Git tag, or publish a release.
The [docs repository](https://github.com/limanix/docs) owns Sphinx configuration, the theme, HTML builds, and site publication.

## Prepare a contribution

1. Update the implementation and the tests that exercise its changed behavior.
2. Update the relevant guide or example. Regenerate references after changing
   commands, flags, or configuration fields.
3. Run the relevant checks and native build. For VM behavior, record the manual
   scenario you tried and its result.
4. Review the diff for generated archives, private VM configurations, and files
   unrelated to the change.
5. Open a labeled PR and describe the user-visible result and validation.

For a first end-to-end scenario, follow [Getting started](getting-started.md).
For guest module development, start with [Choose and manage modules](modules.md).
