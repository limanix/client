# Limanix

[![License: Apache-2.0](https://img.shields.io/github/license/limanix/client?label=license)](LICENSE)

<p align="center">
  <img src=".github/assets/readme-header.png"
       alt="Limanix"
       width="800">
</p>

Linux development environments on macOS, configured with TOML and NixOS modules.

Keep your project and editor on your Mac. Run Linux tools, services, and builds
inside a VM. Limanix integrates Lima for virtualization and NixOS for guest
configuration; you do not need a separate Lima or Nix installation on the host.

[Client guide](docs/index.md) ·
[Releases](https://github.com/limanix/client/releases) ·
[Module catalog](https://github.com/limanix/modules) ·
[Release process](https://github.com/limanix/docs/tree/main/docs/pages/releases)

## Start here

1. [Install the client](docs/installation.md). The current build targets macOS 26
   or newer. Use the native binary: `limanix-arm64` on Apple Silicon or
   `limanix-amd64` on Intel.
2. Follow [Getting started](docs/getting-started.md) to create a small guest,
   enter its shell, add tools, and share your project.
3. Keep the [complete project example](docs/examples/project.toml) beside your
   code and adapt it using the [configuration guide](docs/configuration.md).

No Nix language knowledge is needed for the first environment. The
[concepts guide](docs/concepts.md) explains the host, guest, mounts, and modules.

## Everyday commands

On your Mac, after preparing `limanix.toml` with a VM named `dev-box`:

```console
limanix create --config limanix.toml
limanix shell dev-box
```

Exit the guest shell to return to your Mac. Apply configuration changes and
manage the VM from there:

```console
limanix update --config limanix.toml
limanix list
limanix stop dev-box
limanix start dev-box
```

An update restarts the VM. A read-write project mount exposes the same files on
both systems. Deleting the VM removes its disk but preserves the managed home
by default. Read [Storage and recovery](docs/storage-and-recovery.md) before
removing an environment that contains data you need.

## Find the right guide

| Topic | Guide |
| --- | --- |
| Resources, mounts, users, environment | [Configuration](docs/configuration.md) |
| Bundled tools and custom NixOS modules | [Modules](docs/modules.md) |
| Guest IP, service ports, QEMU setup | [Networking](docs/networking.md) |
| Updates, shell commands, and status | [Work with VMs](docs/working-with-vms.md) |
| Failed creation, updates, and connections | [Troubleshooting](docs/troubleshooting.md) |
| Exact command help and generated field tables | [Reference](docs/reference.md) |
| Build, test, and contribute | [Development](docs/development.md) |

The client owns these guides and generates its CLI and configuration references.
The [docs repository](https://github.com/limanix/docs) assembles them with the
matching module documentation into the Sphinx site.
