# Choose and manage modules

A module adds packages or configures services inside your VM. You select modules in
`limanix.toml`; Limanix applies them when you create or update the VM.

There are two sources:

| Selector | Source | How it becomes available |
| --- | --- | --- |
| `lmx:NAME` | The catalog embedded in your client binary | Install a client containing that module |
| `lmx:NAME-VERSION` | A versioned entry in the same catalog | Choose a selector listed by your client |
| `third-party:NAME` | A local directory you imported | Run `limanix modules add NAME DIRECTORY` |

`lmx:` and `third-party:` identify the source. Neither prefix is a filesystem path
or a GitHub repository URL.

## Select bundled modules

On your **Mac**, inspect the installed client's catalog:

```console
limanix modules list
limanix modules list --json
```

Listing reads the bundled catalog and local imports. It does not download a new
catalog. The JSON form includes each entry's name, source, description, and error;
one damaged import can be reported without hiding the healthy entries.

Edit the existing `[nixos]` table in your configuration:

```toml
[nixos]
modules = ["lmx:git", "lmx:nodejs"]
```

`lmx:nodejs` selects the default defined by that catalog. A selector such as
`lmx:nodejs-24` selects a specific toolchain line **only when your client's list
contains it**. It does not freeze every package across future catalog releases.

Apply the selection on your **Mac**:

```console
limanix update --config limanix.toml
```

For a new VM, use `limanix create --config limanix.toml` instead.

```{important}
An update restarts the VM. Finish running work first. Installing a new client or
editing TOML does not update an existing guest by itself.
```

An empty list, `modules = []`, selects no optional modules. The Limanix base system
remains. The client accepts repeated selectors and multiple versions; whether
those versions can coexist depends on the modules themselves. Check the module's
documentation before selecting multiple versions of a service.

## Import a small local module

The imported directory needs a regular file named `default.nix` at its root. This
example adds `curl`, `jq`, and `ripgrep` to the guest.

Create `my-tools/default.nix` on your **Mac**:

```nix
{ pkgs, ... }:
{
  environment.systemPackages = [
    pkgs.curl
    pkgs.jq
    pkgs.ripgrep
  ];
}
```

Here, `pkgs` is the package set supplied to the module. The list declares packages
to make available in the guest system. Save the file, then import its directory:

```console
limanix modules add my-tools ./my-tools
limanix modules list
```

The first argument is the **unqualified name**. Use `my-tools`, not
`third-party:my-tools`, with `add` and `remove`. Names start with a lowercase
letter, contain lowercase letters and digits separated by single hyphens, and
have at most 63 characters. `my-tools` and `tools2` are valid; `MyTools`, `my_tools`,
and `tools-` are not.

Add the selector to the existing configuration:

```toml
[nixos]
modules = ["lmx:git", "third-party:my-tools"]
```

Update the VM, replacing `dev-box` with the name in your TOML:

```console
limanix update --config limanix.toml
limanix shell dev-box -- jq --version
limanix shell dev-box -- rg --version
```

Importing checks the directory structure and copies the files. **It does not
evaluate the Nix code.** Syntax errors, missing packages, or conflicting options
surface when the guest configuration is built.

For NixOS concepts, service examples, and reusable options, continue with the
[module authoring guide](https://github.com/limanix/modules/tree/main/docs).

## Understand the copies

Limanix does not keep a live link to your source directory:

```{mermaid}
flowchart TD
    Source["Your module directory"] -->|"modules add"| Registry["Local registry copy"]
    Registry -->|"create or update"| VM["VM configuration snapshot"]
    VM -->|"guest build"| System["Applied NixOS system"]
```

| Action | Result |
| --- | --- |
| Edit or delete the original directory | The imported copy and existing VMs stay unchanged |
| Import under another name | A separate registry entry becomes available |
| Remove an imported entry | Existing VM snapshots remain; future updates cannot select the missing entry |
| Reimport changed files and update a VM | That VM receives a new copy of the module |

The registry belongs to the selected [Limanix state directory](storage-and-recovery.md).
An import made with one `LIMANIX_HOME` is not available when using another.

### Keep imports self-contained

The whole directory is copied, including nested files and directories. Relative
imports such as `imports = [ ./extra.nix ];` work when `extra.nix` is inside that
directory.

- Keep files referenced by the module inside its directory. Sibling directories
  outside it are not included in the copy.
- Use regular files and directories. Symlinks inside the tree, including a
  symlinked `default.nix`, and special files such as sockets or FIFOs are rejected.
- The source argument may be relative, absolute, or use `~`. A symlink in the path
  to the source directory is resolved before copying.
- Import the module directory itself, rather than an entire checkout containing
  unrelated files.

## Replace an imported module

`add` refuses an existing name. There is no overwrite or `--force` option.
After editing the original files, run on your **Mac**:

```console
limanix modules remove my-tools
limanix modules add my-tools ./my-tools
limanix update --config limanix.toml
```

Between removal and reimport, configurations selecting `third-party:my-tools`
cannot be created or updated. Reimporting does not rebuild any VM automatically;
update each VM that should receive the change.

To try a revision without replacing the current entry, import it as `my-tools-next`,
select `third-party:my-tools-next` in a test VM, and update that VM.

## Remove a module from a VM

Remove its selector from `nixos.modules`, then update the VM. This changes the
guest's declared packages and services; it does not request deletion of project
files or application data.

For a local module you no longer need, remove the registry entry afterwards:

```console
limanix modules remove my-tools
```

`remove` only manages imported entries. Bundled `lmx:` modules remain available
in the client. See [Troubleshooting](troubleshooting.md) when an import is missing
or a guest build fails.
