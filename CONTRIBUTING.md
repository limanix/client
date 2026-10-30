# Contributing to the client

This repository holds `limanix`, the command on the Mac that creates and
updates LimaNix VMs. The
[organization guide](https://github.com/limanix/.github/blob/main/CONTRIBUTING.md)
covers issues, release labels and the license; this guide covers the work in
this repository.

## Set up

You need [Task](https://taskfile.dev) 3.53.1 or newer and Docker. Native builds
also need Go and the Xcode Command Line Tools. `task --list` shows the checks;
they run in the containers CI uses.

## Find your way

| Path | Holds |
| -- | -- |
| `cmd/limanix` | The command's entry point |
| `cmd/bundle-*` | The tools that prepare the embedded catalog, `lmx`, `socket_vmnet` and the Lima guest agents |
| `internal/config` | `limanix.toml`: its model, defaults, validation and the reference of its keys |
| `internal/nixos` | The generated NixOS configuration and the platform modules in `resources/base` |
| `internal/guest` | Commands that run in the VM over SSH, `limanix shell` among them |
| `internal/cli` | Commands, their help and `reference.json` |

## Before you open a pull request

| Step | Command |
| -- | -- |
| Test | `task ci/golang/test modules_version=v<N>` |
| Format, lint and audit | `task ci/golang/fmt`, `task ci/golang/lint`, `task ci/golang/audit` |
| Evaluate the generated configuration for every catalog selector | `task ci/nix/eval modules_version=v<N>` |
| Update the reference after a command or a `limanix.toml` key changes | `task reference/fix modules_version=v<N>` |
| Check Markdown | `task ci/markdown/fmt`; `task markdown/fix` rewrites files |

`modules_source=../modules` bundles a local catalog checkout and
`lmx_source=../lmx` local `lmx` archives instead of the published releases; use
them to check a change that spans repositories before either is released.
`reference.json` is the source of the command and `limanix.toml` pages on
docs.limanix.dev; a test fails when it is out of date.

## Changes that span repositories

| Change | Order |
| -- | -- |
| A catalog option or capability the platform modules use | Release the catalog first: pull requests and releases of the client build with the newest published catalog |
| The configuration that `lmx` reads | Release `lmx`, then move the pin in `internal/nixos/resources/base/lmx.json` together with the platform modules that write it |
| A new theme or prompt | Release the catalog that adds it: the client takes the names of themes and prompts, and the shells each prompt runs in, from the bundled catalog |

A tag `v<version>` on `main` releases the client with the newest catalog, and
every catalog release publishes the client releases that support it again with
that catalog.
