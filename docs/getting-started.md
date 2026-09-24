# Getting started

Create a small Linux environment, enter its shell, then add tools and share your
project. Start with an [installed native client](installation.md).

## 1. Save a configuration

On your **Mac**, make a new working directory:

```console
mkdir limanix-demo
cd limanix-demo
```

Save this as `limanix.toml`. It uses Apple Silicon, creates a development user,
and selects no optional modules or project mounts:

```{literalinclude} examples/minimal.toml
:language: toml
```

You can also {download}`download the file <examples/minimal.toml>`.
On **Intel**, change `arch` to `"amd64"`.

| Setting | Effect |
| --- | --- |
| `name = "dev-box"` | Name used by lifecycle and shell commands |
| `cpu`, `mem`, `disk` | VM resources; `GiB` is the required memory and disk unit |
| `user` | Your Linux account; this example permits passwordless sudo inside the VM |
| `home.root` | Host directory beneath which Limanix creates this VM's managed home |
| `mounts = []` | No extra project directories are shared |
| `env = {}` | No extra environment variables |
| Empty module and port lists | Base guest system with no optional modules or additional firewall openings |

The managed home is still shared even with `mounts = []`. The empty lists and
table are intentional: omitted collections can retain the built-in example's
values. [Configuration](configuration.md) explains these defaults.

`limanix first-config` is another way to write an editable example. It includes
sample mounts that need review and **overwrites** an existing `limanix.toml`.
For this walkthrough, use the explicit file above.

## 2. Create the VM

Run on your **Mac**:

```console
limanix create --config limanix.toml
```

Creation downloads the base image as needed, boots the guest, builds its NixOS
configuration, and restarts it into that configuration. The first run needs
network access and can take longer while dependencies are downloaded or built.

Wait for the command to finish successfully. It prints the VM name and the
managed home's host path. Then inspect the environment:

```console
limanix list
limanix shell dev-box
```

Inside the **VM**, check where you are:

```console
uname -s
whoami
pwd
```

For this configuration, the expected results are `Linux`, `dev`, and `/home/dev`.
Return to your Mac with `exit`.

## 3. Add development tools

On your **Mac**, inspect the catalog embedded in your installed client:

```console
limanix modules list
```

Select entries from that list. For a catalog containing Git and Node.js, replace
the existing module list in `limanix.toml`:

```toml
[nixos]
modules = ["lmx:git", "lmx:nodejs"]
```

Finish any guest work, then apply the change from your **Mac**:

```console
limanix update --config limanix.toml
limanix shell dev-box -- git --version
limanix shell dev-box -- node --version
```

**Updating restarts the VM.** The new tools come from the selected modules;
there is no separate installation command inside the guest for this workflow.

## 4. Share your project

Remove the top-level `mounts = []` line. At the end of the file, add:

```toml
[[mounts]]
mode = "rw"
source = "."
target = "/workspace"
```

Here, `.` means the directory containing the configuration file. It does not
depend on the terminal directory from which you later run `update`.

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

Your `limanix.toml` is visible along with the rest of the directory. Keep editing
files on the Mac and run project commands here. This mount is read-write: changes
and deletions in either system affect the same files.

For a complete project template, use the example in
[Configure an environment](configuration.md). To reach a guest HTTP server from
your browser, follow [Networking](networking.md).

## 5. Pause and return

Exit the guest shell. On your **Mac**:

```console
limanix stop dev-box
limanix start dev-box
limanix shell dev-box
```

Stopping preserves the VM disk and host files. Starting boots the existing
configuration; it does not read your edited TOML or refresh modules.

If you want to remove the demo, read [Storage and recovery](storage-and-recovery.md)
first. Deleting a VM removes its disk, while preserving its managed home by
default.

## Where to go next

- [Configuration](configuration.md): users, resources, mounts, environment, and defaults.
- [Modules](modules.md): bundled toolchains and your own NixOS configuration.
- [Work with VMs](working-with-vms.md): updates, shell commands, and lifecycle states.
- [Troubleshooting](troubleshooting.md): a failed create, update, or connection.
