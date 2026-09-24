# The main concepts

Limanix connects three things: a configuration file on your Mac, a Linux virtual
machine, and a set of NixOS modules that describe its tools and services.

## Mac and VM are separate systems

| Term | Meaning | What you do there |
| --- | --- | --- |
| **Host** | Your Mac, running macOS | Edit files and run `limanix` commands |
| **Guest** | The Linux system inside the VM | Compile, test, and run Linux programs |
| **VM** | The virtual machine containing the guest | Give it CPU, memory, a disk, and a name |
| **Mount** | A host directory made visible at a guest path | Work on the same project files from both systems |

A mount does not copy your project. If you mount the project at `/workspace`,
your editor on macOS and a compiler in Linux see the same files. With a read-write
mount, deleting a file from the guest also deletes the host file.

```{mermaid}
flowchart TD
    subgraph Mac["Mac · host"]
        Editor["Editor and project files"]
        CLI["Limanix CLI"]
        TOML["limanix.toml"]
    end
    subgraph VM["VM · Linux guest"]
        Workspace["/workspace"]
        Tools["Tools and services"]
        NixOS["NixOS system"]
    end
    Editor <-->|"shared mount"| Workspace
    TOML --> CLI
    CLI -->|"create or update"| NixOS
    NixOS --> Tools
```

Throughout these guides, **Mac** means your normal terminal. **VM** means the
shell opened by `limanix shell NAME`. Use `exit` to return to the Mac.

## What Lima and NixOS do

| Component | Job |
| --- | --- |
| **Limanix** | Reads TOML, manages VM operations, and prepares the guest configuration |
| **Lima** | Runs the Linux VM, connects to it, and shares directories |
| **NixOS** | Configures the Linux system from declarations |
| **Nix** | Evaluates those declarations and builds or obtains their dependencies |
| **Nixpkgs** | Supplies packages and NixOS options used by modules |
| **NixOS module** | A Nix file that contributes packages, services, or other system settings |

Lima is integrated into the client. You do not need a separate Lima or Nix
installation on macOS. The [installation guide](installation.md) covers the
additional host requirements when the guest uses a different CPU architecture.

## Describe the result, then apply it

For example, this selection asks the guest to include Git and Node.js:

```toml
[nixos]
modules = ["lmx:git", "lmx:nodejs"]
```

It is a fragment of a configuration, not a complete first-run file. Use the
[getting-started example](getting-started.md) for a complete environment.

Limanix combines the selected modules with its base NixOS configuration. The
base handles guest integration, the development user, mounts, environment, and
firewall settings. A module adds to that system; it does not replace the base.

The configuration is **declarative**: it states which tools and settings should
be present. Editing the file alone does not change a running guest. On your Mac,
apply the new declaration:

```console
limanix update --config limanix.toml
```

This rebuilds the guest configuration and restarts the VM. It does not run on
every shell connection. See [Work with VMs](working-with-vms.md) for the exact
lifecycle and what happens when an operation fails.

## Three kinds of input

| Input | Owns | Example |
| --- | --- | --- |
| Project TOML | VM resources, module selection, mounts, guest environment, ports | `resources.mem = "4GiB"` |
| Bundled catalog | Modules shipped inside this client binary | `lmx:git` |
| Imported module directory | Your own guest configuration | `third-party:my-tools` |

The client copies imported modules into a local registry. A VM receives a new
snapshot of the selected modules during creation or update. Editing an original
module directory does not change either copy automatically.

The client also embeds the base NixOS flake and its lock file. You select or
import modules through the CLI; there is no TOML field for replacing the entire
flake. Start with [Choose and manage modules](modules.md) when you need custom
guest behavior.

## Configuration is not your data

The VM disk holds the Linux system and guest-only files. The development user's
home is a separate directory on your Mac. Project mounts point to other host
directories you chose.

These locations behave differently on deletion:

| Location | Default `limanix delete NAME` |
| --- | --- |
| VM disk | Removed |
| Managed development home | Preserved |
| Mounted project directories | Left in place |

Keeping TOML and modules in Git helps recreate the declared environment. It does
not back up a database, a VM disk, or files in your home. Continue with
[Getting started](getting-started.md), then read
[Storage and recovery](storage-and-recovery.md) before removing a VM.
