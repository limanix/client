# Command and configuration reference

Use the guides to learn the workflow. Use the client's built-in help for the exact
commands and flags supported by the binary you installed.

## Ask the installed client

On your **Mac**:

```console
limanix --version
limanix --help
limanix create --help
limanix shell --help
limanix modules --help
```

| Need | Command or guide |
| --- | --- |
| Create or update a VM | `limanix create --help`, `limanix update --help` |
| Lifecycle and shell access | [Work with VMs](working-with-vms.md) |
| Module selectors and imports | `limanix modules --help`, [Modules](modules.md) |
| Network helper setup | `limanix network --help`, [Networking](networking.md) |
| TOML fields, defaults, and examples | [Configuration](configuration.md) |
| JSON output for scripts | `limanix list --json`, `limanix modules list --json` |

## Generate an editable TOML example

Use a new, empty directory on your Mac:

```console
mkdir config-example
limanix first-config config-example
```

The command writes `config-example/limanix.toml`. Review the architecture, sample
mounts, ports, and environment before using it. It **overwrites an existing file**
with that name; it does not merge configuration or create missing directories.

For a small first-run configuration without sample mounts, use
[Getting started](getting-started.md).

## References for a release

The release assembly adds a CLI reference, a configuration field reference, and
a downloadable generated TOML example. They are produced
from the same command definitions and configuration model as the selected client
version.

The generated example shows model defaults, including example paths. It is not a
record of any VM on your Mac. The configuration field tables also describe those
defaults; the [configuration guide](configuration.md) explains how omitted and
empty collections behave when loading a file.

A local preview of the handwritten guides can be read without generated files.
For generator commands and release assembly, see
[Develop the client](development.md).
