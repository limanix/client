# Installation

Install the client on your **Mac**. The guest runs Linux; there is no separate
Limanix client installation step inside the VM.

## Check the host

The current Taskfile builds for **macOS 26 or newer**. The binary must match your
Mac's hardware architecture:

| Mac | Client binary | Native guest setting |
| --- | --- | --- |
| Apple Silicon | `limanix-arm64` | `arch = "arm64"` |
| Intel | `limanix-amd64` | `arch = "amd64"` |

VM operations reject an Intel client running through Rosetta on Apple Silicon.
Use the native client even when you want an Intel Linux guest.

The client also requires `ssh` in the host's `PATH`. Creating an environment needs
network access to download the base image and the guest's Nix dependencies.

## Install a release binary

When a release is available, open the
[client releases](https://github.com/limanix/client/releases) and download the
binary for your Mac. Release files are named `limanix-arm64` and
`limanix-amd64`.

For a downloaded Apple Silicon binary in `~/Downloads`, run:

```console
mkdir -p ~/.local/bin
install -m 755 ~/Downloads/limanix-arm64 ~/.local/bin/limanix
export PATH="$HOME/.local/bin:$PATH"
limanix --version
limanix --help
```

On Intel, replace the downloaded filename with `limanix-amd64`. If you saved it
elsewhere, use that path. Keep `~/.local/bin` in your shell's `PATH` for future
terminal sessions.

The build applies an **ad-hoc signature** with the virtualization entitlement;
the release workflow does not notarize the binary with Apple. macOS may require
you to approve opening a downloaded application. Check its source before allowing
it; do not disable Gatekeeper for all applications.

If the releases page has no binary yet, use the source build below once its pinned
module catalog is available.

## Build from source

Install Task, the Go version declared in the checkout's `go.mod`, and Xcode
command-line tools. From the client repository on your **Mac**:

```console
task --yes ci/build
mkdir -p ~/.local/bin
install -m 755 bin/limanix-arm64 ~/.local/bin/limanix
export PATH="$HOME/.local/bin:$PATH"
limanix --version
```

Use `bin/limanix-amd64` on Intel. The task prepares the embedded resources, builds
both architectures, signs them, and verifies the signatures. This native build
does not require Docker.

The module tag selected by `modules_version` in `Taskfile.yml` must exist
in `limanix/modules`. A missing tag prevents a clean build from preparing the
catalog. [Develop the client](development.md) explains the pins and build tools.

## Choose a guest architecture

Use the architecture matching your Mac for the
[first environment](getting-started.md). This selects Apple's
Virtualization.framework and its native NAT network.

| Guest architecture | Virtualization | Extra setup |
| --- | --- | --- |
| Matches the Mac | Virtualization.framework | No QEMU or `network setup` required |
| Differs from the Mac | QEMU | External QEMU and the shared network helper |

For a guest with a different architecture, install QEMU and make the executable
for the **guest** available in `PATH`:

| Guest setting | Required executable |
| --- | --- |
| `arch = "amd64"` | `qemu-system-x86_64` |
| `arch = "arm64"` | `qemu-system-aarch64` |

Then run on your Mac:

```console
limanix network setup
```

This installs the bundled `socket_vmnet` helper and its host networking setup;
it may request administrator approval. Read [Networking](networking.md) for
details and diagnostics. Do not run the whole Limanix CLI with `sudo`.

## After installation

Follow [Getting started](getting-started.md) to create a VM with an explicit,
small configuration. If the executable fails before you can create a VM, start
with the host checks in [Troubleshooting](troubleshooting.md).

Replacing the client binary does not rebuild existing guests. To apply the new
client's base configuration or catalog, use `limanix update --config PATH` for
each environment when you are ready for its restart.
