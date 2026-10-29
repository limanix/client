[![Release](https://img.shields.io/github/v/release/limanix/client)](https://github.com/limanix/client/releases)
[![License: Apache-2.0](https://img.shields.io/github/license/limanix/client?label=license)](LICENSE)

<p align="center">
  <img src=".github/assets/readme-header.png"
       alt="LimaNix"
       width="100%">
</p>

<p align="center">
  <a href="https://docs.limanix.dev"><b>Docs</b></a>
  &nbsp;|&nbsp;
  <a href="https://docs.limanix.dev/install"><b>Install</b></a>
  &nbsp;|&nbsp;
  <a href="https://docs.limanix.dev/quick-start"><b>Quick start</b></a>
  &nbsp;|&nbsp;
  <a href="https://docs.limanix.dev/reference/cli"><b>CLI reference</b></a>
  &nbsp;|&nbsp;
  <a href="https://docs.limanix.dev/architecture"><b>How it works</b></a>
  &nbsp;|&nbsp;
  <a href="https://github.com/limanix/client/releases"><b>Releases</b></a>
</p>

<hr>

`limanix`, the Mac side of [LimaNix](https://limanix.dev): it creates, updates
and opens NixOS development VMs from one `limanix.toml`. Lima is built in, and
each release bundles one [catalog](https://github.com/limanix/modules) release
and pins one [`lmx`](https://github.com/limanix/lmx) release. It needs macOS 26
or newer and `ssh` in `PATH`.

```text
┌──────────────┐  reads  ┌───────────────────────┐    SSH    ┌──────────────┐
│ limanix.toml │ ──────> │     limanix (Mac)     │ ────────> │  VM (NixOS)  │
│  your file   │         │ Lima + catalog inside │   mounts  │  lmx, tools  │
└──────────────┘         └───────────────────────┘           └──────────────┘
```

| Command | What it does |
| -- | -- |
| `limanix first-config` | Writes a `limanix.toml` to start from |
| `limanix create --config limanix.toml` | Creates the VM and builds its system |
| `limanix shell example-box` | Opens a shell in the VM; `--session` keeps a named one |
| `limanix update --config limanix.toml` | Applies a changed file and restarts the VM |
| `limanix list`, `doctor`, `network check` | Shows the VMs and checks a VM or one of its ports |
| `limanix stop`, `start`, `delete` | Stops, starts or deletes a VM; its home stays on the Mac |

## Development

You need [Task](https://taskfile.dev) 3.53.1 or newer and Docker; native builds
also need Go and the Xcode Command Line Tools. Checks run in the containers CI
uses.

| Task | Does |
| -- | -- |
| `task ci/golang/test modules_version=v5` | Go tests with the race detector |
| `task ci/golang/lint`, `ci/golang/fmt`, `ci/golang/audit` | Lint, formatting and known vulnerabilities |
| `task ci/nix/eval modules_version=v5` | Evaluates the generated NixOS flake for every catalog selector |
| `task release/build modules_version=v5` | Builds ad-hoc signed `bin/limanix-arm64` and `bin/limanix-amd64` natively |
| `task reference/fix modules_version=v5` | Updates `reference.json`, the commands and `limanix.toml` keys that the LimaNix site reads; a test fails when it is out of date |

`modules_source=../modules` bundles a local catalog checkout, and
`lmx_source=../lmx` the archives that `task release/build` left in
`../lmx/dist`, instead of the published releases.

| Embedded resource | Pinned in | Prepared by |
| -- | -- | -- |
| Module catalog and base disk | `modules_version`, or `modules_source` | `cmd/bundle-modules` |
| Lima guest agents | Lima in `go.mod` | `cmd/bundle-guestagent` |
| `socket_vmnet` | `Taskfile.yml` | `cmd/bundle-socketvmnet` |
| `lmx` | `internal/nixos/resources/base/lmx.json`; the VM downloads it, or `lmx_source` embeds local archives | `cmd/bundle-lmx` |

A tag `v<version>` on `main` releases the client with the newest catalog, and
every catalog release publishes the client releases that support it again with
that catalog.

## Contributing

Follow the
[contribution guide](https://github.com/limanix/.github/blob/main/CONTRIBUTING.md).
Use [Issues](https://github.com/limanix/client/issues) for questions, bug
reports and feature requests. Licensed under [Apache 2.0](LICENSE).
