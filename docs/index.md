# Use Limanix

Keep your editor and project files on your Mac. Run Linux tools, builds, and
services in a VM described by a small TOML file.

You do not need to know the Nix language to create your first environment. Start
with the base system, then select ready-made modules or write your own.

## Choose your starting point

| I want to… | Start here |
| --- | --- |
| Understand what runs where | [The main concepts](concepts.md) |
| Install the command-line client | [Installation](installation.md) |
| Create my first Linux environment | [Getting started](getting-started.md) |
| Share a project and configure resources | [Configure an environment](configuration.md) |
| Add a language, tool, or service | [Choose and manage modules](modules.md) |
| Connect to a server inside the VM | [Networking](networking.md) |
| Start, stop, update, or remove a VM | [Work with VMs](working-with-vms.md) |
| Know which files survive deletion | [Storage and recovery](storage-and-recovery.md) |
| Diagnose a failed command | [Troubleshooting](troubleshooting.md) |
| Change the client itself | [Develop the client](development.md) |

## The everyday loop

```{mermaid}
flowchart TD
    Config["Describe the environment"] --> Create["Create the VM"]
    Create --> Work["Edit on Mac · run in Linux"]
    Work --> Change["Change TOML or modules"]
    Change --> Update["Update and restart the VM"]
    Update --> Work
```

The TOML file describes the environment. Your project and application data have
their own storage lifecycle. Read [Storage and recovery](storage-and-recovery.md)
before deleting an environment you have used for real work.

## Find an exact command or field

[Command and configuration reference](reference.md) explains the built-in help
and the references generated from the client source. The release documentation
includes the reference for that exact client version.

```{toctree}
:hidden:
:caption: Start here
:maxdepth: 1

concepts
installation
getting-started
```

```{toctree}
:hidden:
:caption: Use your environment
:maxdepth: 1

configuration
modules
networking
working-with-vms
storage-and-recovery
troubleshooting
```

```{toctree}
:hidden:
:caption: Reference and development
:maxdepth: 1
:glob:

reference**
development
```
