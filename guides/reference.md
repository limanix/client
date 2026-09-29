# Reference

The reference pages contain exact commands and configuration fields in the built documentation.
In GitHub's source view, they point to handwritten guides and the installed client's help.

| Page                                                  | Contents                                               |
|-------------------------------------------------------|--------------------------------------------------------|
| [CLI reference](reference/cli.md)                     | Commands, arguments, flags, and help text              |
| [Configuration reference](reference/configuration.md) | Field types, defaults, and a downloadable TOML example |

Their detailed content is generated from the client's command definitions and configuration model.
For preparation commands, see [Development](development.md#maintain-the-documentation).

## Help for the installed client

Run on your Mac to check the version and commands available in your binary:

```console
limanix --version
limanix --help
limanix create --help
limanix shell --help
limanix modules --help
limanix network --help
```

`limanix list --json` and `limanix modules list --json` provide JSON output for scripts.

## Generate a configuration example

Create a directory and write the client's example into it:

```console
mkdir config-example
limanix first-config config-example
```

The command writes `config-example/limanix.toml`.
It **overwrites an existing file** with that name and does not create missing directories.
Review the architecture, mounts, ports, and environment before using it.

The example contains model defaults and sample paths, not the configuration of an existing VM.
[Configuration](configuration.md#understand-omitted-values) explains how omitted settings and empty collections behave.
For a minimal first environment, use [Getting started](getting-started.md).

```{toctree}
:hidden:

reference/cli
reference/configuration
```
