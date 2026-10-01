# Modules

Use the module commands on your **Mac** to list the catalog and manage imported module directories.
Select modules in `limanix.toml` and apply them when you create or update a VM.
Each selected module adds to the client's base NixOS configuration.

| Selector                         | Source                                     | How it becomes available                  |
|----------------------------------|--------------------------------------------|-------------------------------------------|
| `lmx:NAME` or `lmx:NAME-VERSION` | The catalog embedded in your client binary | Install a client containing that selector |
| `third-party:NAME`               | A local directory in your module registry  | Run `limanix modules add NAME DIRECTORY`  |

For available tools and version behavior, see [Catalog](https://limanix.dev/categories/nixos/catalog.html).
To write Nix code, start with [Write a module](https://limanix.dev/categories/nixos/writing-modules.html).

Modules read the guest identity through the read-only NixOS options `config.limanix.user.name` and `config.limanix.user.home`.
Set `limanix.user.shell` in a custom Nix module to choose the login shell.
The existing `runtime` argument remains available for compatibility.

## List available modules

```console
limanix modules list
limanix modules list --json
```

Listing reads the bundled catalog and local registry; it does not download a new catalog or evaluate Nix code.
The JSON output is an array with these fields:

| Field         | Value                                                                                      |
|---------------|--------------------------------------------------------------------------------------------|
| `name`        | Full selector, such as `lmx:git` or `third-party:my-tools`                                 |
| `source`      | `lmx` or `third-party`                                                                     |
| `description` | Catalog description or `Locally imported NixOS module`                                     |
| `error`       | `null` when the entry passes registry checks, or a diagnostic string for an invalid import |

The `error` field is always present.
A damaged import can be reported alongside healthy entries without failing the whole listing.

## Select modules for a VM

Edit the existing `[nixos]` table in `limanix.toml`:

```toml
[nixos]
modules = ["lmx:git", "third-party:my-tools"]
```

Only select `third-party:my-tools` after importing it as described below.
An empty list, `modules = []`, selects no optional modules; the LimaNix base system and public catalog declarations remain available.
Repeated selectors are accepted.

Standard modules can compose other catalog modules using Nix imports.
For example, `lmx:console` includes the configured shell, editor, session manager and terminal tools.
The client keeps their source paths together so selecting a component separately refers to the same module.
The client loads public declarations from `catalog/_shared/<area>.nix` even when no standard modules are selected.
Private declarations under `_shared/internal/` are available for explicit catalog imports and are not loaded automatically.
The `_shared` directory has no selector; `capabilities` and `internal` are reserved catalog module names.
Third-party modules can use public capability options without importing catalog source files.
The [catalog contract](https://limanix.dev/categories/nixos/catalog-contract.html) defines the public namespaces and compatibility rules.

Apply a changed selection with `limanix update --config limanix.toml`, or use `limanix create --config limanix.toml` for a new VM.
Read [Apply a configuration change](virtual-machines.md#apply-a-configuration-change) before updating an existing VM.
Editing TOML or installing another client does not change an existing guest by itself.

## Import a module

The directory must contain a regular file named `default.nix` at its root.
To register an existing module saved in `./my-tools` under the name `my-tools`, run:

```console
limanix modules add my-tools ./my-tools
limanix modules list
```

This makes `third-party:my-tools` available for selection in `nixos.modules`.
The import always uses `default.nix`; `third-party:` does not expose catalog version selectors.

```{important}
Importing checks the directory structure and copies the files.
It does not evaluate the Nix code.
Syntax errors, missing packages, or conflicting options surface when the guest configuration is built.
```

### Module names

Use the unqualified name with `add` and `remove`: `my-tools`, not `third-party:my-tools`.

| Rule                                                             | Examples                                                                      |
|------------------------------------------------------------------|-------------------------------------------------------------------------------|
| Start with a lowercase letter                                    | `tools2` is valid; `2tools` is not                                            |
| Use lowercase letters, digits, and single hyphens between groups | `my-tools` is valid; `MyTools`, `my_tools`, `my--tools`, and `tools-` are not |
| Use at most 63 characters                                        | The limit applies to the name without `third-party:`                          |

### Keep imports self-contained

The whole directory is copied, including hidden files and directories such as `.git`.

- Keep files referenced by relative imports inside the module directory.
  Sibling directories outside it are not included in the copy.
- Use regular files and directories.
  Symlinks inside the tree, including a symlinked `default.nix`, and special files such as sockets or FIFOs are rejected.
- The source argument may be relative, absolute, or use `~`.
  Symlinks in the path to the source directory are resolved before copying.
- Import the module directory itself, rather than an entire checkout containing unrelated files.

## Understand the copies

Limanix does not keep a live link to your source directory:

```{mermaid}
flowchart TD
    Source["Your module directory"] -->|"modules add"| Registry["Local registry copy"]
    Registry -->|"create or update"| VM["VM configuration snapshot"]
    VM -->|"guest build"| System["Applied NixOS system"]
```

| Action                                 | Result                                                                                   |
|----------------------------------------|------------------------------------------------------------------------------------------|
| Edit or delete the original directory  | The imported copy and existing VMs stay unchanged                                        |
| Import under another name              | A separate registry entry becomes available                                              |
| Remove an imported entry               | Existing VM snapshots remain; future creates and updates cannot select the missing entry |
| Reimport changed files and update a VM | That VM receives a new copy of the module                                                |

The registry belongs to the selected [LimaNix state directory](troubleshooting.md#state-directories).
An import made with one `LIMANIX_HOME` is not available when using another.

## Replace an imported module

`add` refuses an existing name.
There is no overwrite or `--force` option.
After editing the original files, run:

```console
limanix modules remove my-tools
limanix modules add my-tools ./my-tools
limanix update --config limanix.toml
```

Between removal and reimport, configurations selecting `third-party:my-tools` cannot be created or updated.
Reimporting does not rebuild any VM automatically; update each VM that should receive the change.

To try a revision without replacing the current entry:

1. Run `limanix modules add my-tools-next ./my-tools`.
2. Replace `third-party:my-tools` with `third-party:my-tools-next` in a test VM's configuration.
3. Update that test VM and check the result.

## Remove a module from a VM

Remove its selector from `nixos.modules`, then update the VM.
This changes the guest's declared packages and services; it does not request deletion of project files or application data.

For a local module you no longer need, remove the registry entry afterwards:

```console
limanix modules remove my-tools
```

`remove` only manages imported entries; bundled `lmx:` modules remain available in the client.
Removing a registry entry does not edit any VM's `nixos.modules` selection or change its applied system.
For failures while applying a module, follow [Create or update failed during provisioning](troubleshooting.md#create-or-update-failed-during-provisioning).
