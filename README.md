# LimaNix client

[![License: Apache-2.0](https://img.shields.io/github/license/limanix/client?label=license)](LICENSE)

<p align="center">
  <img src=".github/assets/readme-header.png"
       alt="LimaNix"
       width="100%">
</p>

LimaNix creates and manages Linux development environments on macOS. Describe
your VM in `limanix.toml`, select NixOS modules, and run Linux tools and
services while keeping your project files on your Mac.

The client manages VM lifecycle, configuration and shell access through Lima.
Each binary bundles one [module catalog](https://github.com/limanix/modules)
release, which provides guest software, application configuration and
integrations. You do not need to install Lima or Nix separately on your Mac.

[Documentation](https://limanix.dev/categories/client/index.html) |
[Getting started](guides/getting-started.md) |
[Releases](https://github.com/limanix/client/releases)

## Get started

LimaNix requires macOS 26 or newer and `ssh` in your `PATH`. Follow
[Install the client](guides/getting-started.md#install-the-client) using
`limanix-arm64` for Apple Silicon or `limanix-amd64` for Intel.

Save the [minimal example](guides/examples/minimal.toml) as `limanix.toml`. On
Intel, change `resources.arch` to `"amd64"`. Run on your Mac:

```console
limanix create --config limanix.toml
limanix shell dev-box
```

`limanix shell` opens a shell in the new NixOS VM. Exit the guest shell to
return to your Mac. Follow [Getting started](guides/getting-started.md) to add
tools, share your project and apply configuration changes. For the integrated
development workbench, use the
[Cozy example](guides/workspace.md#create-the-workbench).

## Documentation

| Guide | Contents |
| -- | -- |
| [Getting started](guides/getting-started.md) | Installation and your first environment. |
| [Project workspace](guides/workspace.md) | Cozy workbench, project windows and integrated tools. |
| [Configuration](guides/configuration.md) | Resources, users, mounts, environment and defaults. |
| [Modules](guides/modules.md) | Catalog selectors, imported modules and applying changes. |
| [Networking](guides/networking.md) | Service ports, guest architecture and connection checks. |
| [Virtual machines](guides/virtual-machines.md) | Shell access, updates, lifecycle and storage. |
| [Architecture](guides/architecture.md) | Responsibilities, configuration delivery and recovery. |
| [Troubleshooting](guides/troubleshooting.md) | Failed commands, guest access and saved state. |
| [Reference](guides/reference.md) | CLI commands, configuration fields and defaults. |

The guides and generated references are published at
[limanix.dev](https://limanix.dev/categories/client/index.html). The
[docs repository](https://github.com/limanix/docs) assembles them with the
module documentation.

## Contributing

Follow the
[contribution guide](https://github.com/limanix/.github/blob/main/CONTRIBUTING.md)
when changing the client. See [Development](guides/development.md) and
[Taskfile.yml](Taskfile.yml) for prerequisites, local checks, native builds and
documentation tasks. Use [Issues](https://github.com/limanix/client/issues) for
questions, bug reports and feature requests.

Licensed under [Apache 2.0](LICENSE).
