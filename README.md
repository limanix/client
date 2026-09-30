# LimaNix

[![License: Apache-2.0](https://img.shields.io/github/license/limanix/client?label=license)](LICENSE)

<p align="center">
  <img src=".github/assets/readme-header.png"
       alt="LimaNix"
       width="100%">
</p>

Linux development environments on macOS, configured with TOML and NixOS modules.

Keep your project and editor on your Mac.
Run Linux tools, services, and builds inside a VM.
LimaNix uses Lima for virtualization and NixOS for guest configuration.
The host does not need a separate Lima or Nix installation.

[Documentation](https://limanix.dev) |
[Releases](https://github.com/limanix/client/releases) |
[Module catalog](https://github.com/limanix/modules) |
[Release process](https://limanix.dev/releases/index.html)

## How it compares

LimaNix combines VM settings and NixOS module selection in a project's TOML file.
The selected tools run as Linux builds inside the VM, without requiring Nix on the Mac.
Read [How LimaNix compares](https://limanix.dev/comparison.html) for details and trade-offs.

## Install

LimaNix requires macOS 26 or newer and `ssh` in your `PATH`.
Download the binary from [Releases](https://github.com/limanix/client/releases).
The command below uses `limanix-arm64`; replace it with `limanix-amd64` on Intel.

```console
mkdir -p ~/.local/bin
install -m 755 ~/Downloads/limanix-arm64 ~/.local/bin/limanix
export PATH="$HOME/.local/bin:$PATH"
limanix --version
```

Use the native binary.
VM operations reject an Intel client running through Rosetta.
Release binaries carry an ad-hoc signature and are not notarized.
macOS may ask you to approve the download.
See [Install the client](guides/getting-started.md#install-the-client) for details.

## Create an environment

Save the [minimal example](guides/examples/minimal.toml) as `limanix.toml` beside your project.
On Intel, set its `arch` to `"amd64"`.
Run the commands on your Mac:

```console
limanix create --config limanix.toml
limanix shell dev-box
```

Exit the guest shell to return to your Mac.
Apply configuration changes and manage the VM from there:

```console
limanix update --config limanix.toml
limanix list
limanix stop dev-box
limanix start dev-box
```

An update restarts the VM.
A read-write project mount exposes the same files on both systems.
Deleting the VM removes its disk but preserves the managed home by default.
Read [Storage and data](guides/virtual-machines.md#storage-and-data) before removing an environment that contains data you need.

[Getting started](guides/getting-started.md) walks through the first environment.
The [complete project example](guides/examples/project.toml) shows every configuration section.

## Add tools

Select modules from the [catalog](https://github.com/limanix/modules) in the `[nixos]` table.
Run `limanix update --config limanix.toml` after editing the configuration:

```toml
[nixos]
modules = ["lmx:go", "lmx:docker"]
```

`limanix modules list` shows the selectors your client provides.
The catalog does not require Nix knowledge.
For software the catalog does not cover, [write a module](https://limanix.dev/categories/nixos/writing-modules.html).
Then import it with `limanix modules add`.

## Versions

Client releases use tags such as `v0.0.1`.
Each binary embeds one module catalog release.
When a new catalog is published, recent client versions are rebuilt with it.
The resulting builds use tags such as `v0.0.1+1`.
Their release notes name the bundled catalog.
`limanix --version` prints the build you have.

## Find the right guide

| Topic                                                          | Guide                                          |
|----------------------------------------------------------------|------------------------------------------------|
| Installation and first environment                             | [Getting started](guides/getting-started.md)   |
| Resources, mounts, users, environment, modules, and networking | [Configuration](guides/configuration.md)       |
| Catalog selectors and imported modules                         | [Modules](guides/modules.md)                   |
| Service ports, guest architecture, and connection checks       | [Networking](guides/networking.md)             |
| Shell access, updates, status, storage, and deletion           | [Virtual machines](guides/virtual-machines.md) |
| Failed creation, updates, and connections                      | [Troubleshooting](guides/troubleshooting.md)   |
| Exact command help and generated field tables                  | [Reference](guides/reference.md)               |
| Build, test, and contribute                                    | [Development](guides/development.md)           |

The client owns these guides and generates its CLI and configuration references.
The [docs repository](https://github.com/limanix/docs) assembles the client guides with the matching module documentation.
It publishes the site at [limanix.dev](https://limanix.dev).

## Develop

Checks run in containers.
They require [Task](https://taskfile.dev) 3.53.1 or newer and Docker with a running engine.
Native builds run on macOS with the Go version from `go.mod` and the Xcode command-line tools.

```console
task --yes ci/golang-fmt ci/golang-lint ci/golang-vuln
task --yes ci/golang-test modules_version=v2
task --yes ci/build modules_version=v2
```

`modules_version` sets the module catalog tag to embed.
Use the latest tag from [catalog releases](https://github.com/limanix/modules/releases).
`ci/build` writes `bin/limanix-arm64` and `bin/limanix-amd64`.
See [Development](guides/development.md) for source layout, embedded resources, and documentation tasks.
Read the [contribution guide](https://github.com/limanix/.github/blob/main/CONTRIBUTING.md) before opening a pull request.

Licensed under [Apache 2.0](LICENSE).
