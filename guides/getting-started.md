# Getting started

Install the client, create a Linux VM, then add tools and share your project.
LimaNix reads your TOML configuration, uses Lima to run the VM, and configures
its Linux system with NixOS. You do not need to know Nix or install Lima or Nix
separately on macOS.

In these guides, **Mac** means your host terminal; **VM** means the Linux guest
shell opened by `limanix shell`. Keep your editor and project files on the Mac,
and run Linux tools inside the VM.

## Install the client

The current client build requires **macOS 26 or newer** and `ssh` in the Mac's
`PATH`. Download the binary for your Mac from
[Client releases](https://github.com/limanix/client/releases):

| Mac | Binary | Guest architecture for this guide |
| -- | -- | -- |
| Apple Silicon | `limanix-arm64` | `arm64` |
| Intel | `limanix-amd64` | `amd64` |

VM operations reject an Intel client running through Rosetta on Apple Silicon.
Use the native client even when you need a guest with another architecture.

For an Apple Silicon binary saved in `~/Downloads`, run on your **Mac**:

```console
mkdir -p ~/.local/bin
install -m 755 ~/Downloads/limanix-arm64 ~/.local/bin/limanix
export PATH="$HOME/.local/bin:$PATH"
limanix --version
limanix --help
```

On Intel, use `~/Downloads/limanix-amd64`; adjust the source path if you saved
the file elsewhere. Keep `~/.local/bin` in your shell's `PATH` for future
terminal sessions.

The build applies an **ad-hoc signature** with the virtualization entitlement;
the release workflow does not notarize the binary with Apple. If macOS asks you
to approve opening the download, check its source before allowing it. For a
source build, or if no release binary is available, follow
[Build a native client](development.md#build-a-native-client).

This guide uses a guest architecture matching your Mac, which requires no QEMU
or `limanix network setup`. For a different guest architecture, follow
[Choose the guest architecture](networking.md#choose-the-guest-architecture)
before creating the VM. Run LimaNix as your normal Mac user, without `sudo`.

## Know where your files live

This guide uses three kinds of storage:

| Place | Where the files are | After `limanix delete` |
| -- | -- | -- |
| Guest disk | Inside the VM, outside shared directories; holds NixOS and data written outside mounts | Removed |
| Managed home | On the Mac beneath `~/.limanix`; appears at `/home/dev` inside the VM | Kept by default; `--remove-home` removes it |
| Project mount | A Mac directory that appears at `/workspace` after you add the mount | Kept on the Mac |

The managed home and project mount share files with the Mac; edits or deletions
inside the VM affect those files on the Mac. The first configuration has no
project mount; you add one later. The
[storage guide](virtual-machines.md#storage-and-data) explains what to preserve
before deleting a VM.

## Save a configuration

On your **Mac**, make a new working directory:

```console
mkdir limanix-demo
cd limanix-demo
```

Save this as `limanix.toml`. It targets Apple Silicon, creates a development
user, and selects no optional modules or project mounts:

```toml
schema_version = 1
name = "dev-box"
mounts = []
env = {}

[user]
name = "dev"
home = "/home/dev"
sudo = true

[resources]
arch = "arm64"
cpu = 2
mem = "4GiB"
disk = "16GiB"

[home]
root = "~/.limanix"

[nixos]
modules = []

[network]
mode = "shared"

[network.ports]
tcp = []
udp = []
```

You can also [open or download the TOML file](examples/minimal.toml). On
**Intel**, change `arch` to `"amd64"`.

| Setting | Effect |
| -- | -- |
| `name = "dev-box"` | Name used by lifecycle and shell commands |
| `cpu`, `mem`, `disk` | VM resources; `GiB` is the required memory and disk unit |
| `user` | Your Linux account; this example permits passwordless sudo inside the VM |
| `home.root` | Host directory beneath which Limanix creates this VM's managed home |
| `mounts = []` | No extra project directories are shared |
| `env = {}` | No extra environment variables |
| Empty module and port lists | Base guest system with no optional modules or additional firewall openings |

The managed home is still shared even with `mounts = []`. The empty lists and
table are intentional: omitted collections can retain the built-in example's
values. [Configuration](configuration.md#understand-omitted-values) explains
these defaults.

`limanix first-config` writes a different example with sample mounts and
**overwrites** an existing `limanix.toml`. Use the explicit file above for this
walkthrough.

## Create the VM

Run on your **Mac**:

```console
limanix create --config limanix.toml
```

Creation downloads the base image as needed, boots the guest, builds its NixOS
configuration, and restarts it into that configuration. The first run needs
network access for the image and Nix dependencies.

After the command succeeds and prints the VM name and managed home's host path,
inspect the environment:

```console
limanix list
limanix shell dev-box
```

Inside the **VM**, check where you are:

```console
uname -s
whoami
pwd
limanix-help
limanix-info
```

The first three commands return `Linux`, `dev`, and `/home/dev` for this
configuration. `limanix-help` shows guest and Mac commands; `limanix-info`
reports the guest identity, selected modules, shared mounts and failed services.
These local helpers work without optional modules. Return to your Mac with
`exit`.

## Add development tools

On your **Mac**, inspect the catalog embedded in your installed client:

```console
limanix modules list
```

Select entries from that list. If it contains Git and Node.js, replace only the
`modules = []` line under the existing `[nixos]` table in `limanix.toml` with:

```toml
modules = ["lmx:git", "lmx:nodejs"]
```

Finish any guest work, then apply the change from your **Mac**:

```console
limanix update --config limanix.toml
limanix shell dev-box -- git --version
limanix shell dev-box -- node --version
```

**Updating restarts the VM.** The selected modules add tools to the base NixOS
system; there is no separate installation step inside the guest. Editing TOML or
replacing the client binary alone does not change an existing VM.

## Share your project

Remove the top-level `mounts = []` line. At the end of the file, add:

```toml
[[mounts]]
mode = "rw"
source = "."
target = "/workspace"
```

Here, `.` means the directory containing the configuration file, regardless of
the terminal directory where you run `update`.

On your **Mac**:

```console
limanix update --config limanix.toml
limanix shell dev-box
```

Inside the **VM**:

```console
cd /workspace
ls
```

Your `limanix.toml` is visible along with the rest of the directory. Keep
editing files on the Mac and run project commands here. This read-write mount
shares the same files: changes and deletions in either system affect both.

For a complete project template, see [Configuration](configuration.md). To reach
a guest HTTP server from your browser, follow [Networking](networking.md).

## Stop and resume

Exit the guest shell, then run on your **Mac**:

```console
limanix stop dev-box
limanix start dev-box
limanix shell dev-box
```

Stopping preserves the VM disk and host files. Starting boots the existing
configuration; it does not read your edited TOML or refresh modules.

To remove the demo, follow [Delete a VM](virtual-machines.md#delete-a-vm).
Deletion removes the VM disk and preserves its managed home by default; mounted
project directories remain on your Mac.

## Where to go next

For the integrated shell, editor, containers, language servers and cloud
clients, follow [Project workspace](workspace.md).

- [Configuration](configuration.md): users, resources, mounts, environment, and
  defaults.
- [Modules](modules.md): bundled tools and your own NixOS configuration.
- [Virtual machines](virtual-machines.md): updates, shell commands, storage, and
  lifecycle states.
- [Troubleshooting](troubleshooting.md): a failed create, update, or connection.
