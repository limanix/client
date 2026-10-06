# Virtual machines

Use `create` for a new VM, `update` for configuration changes, and `start` /
`stop` for everyday power control. Run the commands on your **Mac**. The
examples use a VM named `dev-box`; replace it with your VM's name.

## Commands

| Command | Result |
| -- | -- |
| `limanix create --config limanix.toml` | Allocate a VM and managed home, build NixOS, restart, and check the development account. |
| `limanix update --config limanix.toml` | Keep the VM identity and storage, apply the configuration, and restart. |
| `limanix start dev-box` | Start the saved VM without rereading TOML or rebuilding NixOS. |
| `limanix stop dev-box` | Shut down the VM and keep its disk and managed home. |
| `limanix shell dev-box` | Open a terminal as the development user in their home. |
| `limanix list` | Show backend power state, the last Limanix operation, and the discovered guest address. |
| `limanix delete dev-box` | Remove the VM disk and saved VM record; preserve the managed home. |

`create` and `update` use `name` from the TOML file; the other VM commands take
the name directly. Changing `name` selects a different VM; it does not rename an
existing one. Editing the file or installing another client does not change an
existing guest until you run `update`.

## Open a shell

```console
limanix shell dev-box
```

With the defaults, you enter the guest as `dev` in `/home/dev`. Interactive
sessions use the development account's configured login shell: Bash by default,
or Zsh when selected through the catalog. Use `cd /workspace` if you mounted
your project there. Type `exit` to return to your Mac; the VM keeps running.

To keep terminal work running when the connection closes, select a session
provider in the VM configuration and apply it with
`limanix update --config limanix.toml`. See the catalog's
[Tmux module](https://limanix.dev/categories/nixos/modules/tmux/README.html) for
one provider and its bindings. Open a named session:

```console
limanix shell dev-box --session work
```

The selected provider creates the session if needed or attaches to the existing
one. The tmux provider rejects names containing `.` or `:`; use names such as
`work` or `project-api`. Detachment, restoration, and key bindings belong to the
provider; follow its module documentation. Stopping or updating the VM
interrupts running processes. If no provider is configured, the guest reports
available module selectors and exits with status 127. `--session` takes a
nonempty name and cannot be combined with a guest command. The flag also works
before the VM name: `limanix shell --session work dev-box`.

To run one guest command and return its exit status to your Mac:

```console
limanix shell dev-box -- id
limanix shell dev-box -- pwd
```

Commands also start in the development user's home. LimaNix passes arguments
through; use a guest shell for `cd`, pipelines, or variable expansion:

```console
limanix shell dev-box -- bash -lc 'cd /workspace && pwd'
limanix shell dev-box -- bash -lc 'printf "%s\n" "$HOME"'
```

Single quotes keep the Mac's shell from expanding `$HOME` before it reaches the
VM.

## Read the status

```console
limanix list
limanix list --json
```

`STATUS` describes the Lima backend; `STATE` describes the last Limanix
operation. `DISK` shows how much of the guest disk is used, by bytes or, when
they are scarcer, by inodes, for example `97% inodes`; `-` means a stopped VM or
an unreadable guest. `list --json` reports the same values in `disk`.

| Example | Meaning |
| -- | -- |
| `Running` / `ready` | The backend is running; the last create or update completed. |
| `Stopped` / `ready` | Configuration was applied successfully; the VM is now stopped. |
| `Running` / `error` | The backend is running, but a LimaNix operation failed. |
| Any status / `interrupted` | A create, update, or delete record remained after its operation lock was released. |
| `Missing` | No backend matched the saved identity, or the identity could not be read. |
| Any status / `corrupt` | LimaNix could not read a valid operation record. |

Active operations show `creating`, `updating`, or `deleting`. Starting or
stopping a VM does not reset an earlier operation error to `ready`. See
[Troubleshooting](troubleshooting.md) for error, interrupted, missing, or
corrupt records.

`ready` is a saved result, not a continuous health check. Check current guest
access with:

```console
limanix shell dev-box -- true
```

A successful exit confirms access as the development user; check application
health separately. An `ADDRESS` of `-` means no shared-network IPv4 address was
discovered. See [Networking](networking.md) for service access.

## Apply a configuration change

1. Save running work: the update interrupts guest sessions and services.

1. Edit the TOML file using [Configuration](configuration.md) and
   [Modules](modules.md).

1. Apply it from your Mac:

   ```console
   limanix update --config limanix.toml
   ```

1. After success, reconnect and check the tool or service you changed.

```{mermaid}
flowchart TD
    A["Check configuration and prepare inputs"] --> B["Stop VM if running; apply Lima settings"]
    B --> C["Start guest and build NixOS"]
    C --> D["Restart, check user, record ready"]
    D --> E["Remove replaced generations and unused store paths"]
```

The NixOS build prepares the next boot. LimaNix then restarts the guest and
checks that the development user can run a command. A successful update leaves
the VM running, even if it was stopped before. After recording the update,
LimaNix removes the previous NixOS generations and the store paths only they
used; a failure there is reported as a warning, and the update stays successful.

| Can change through `update` | Requires a new VM |
| -- | -- |
| CPU, memory, and disk growth | Guest architecture |
| Modules, environment, firewall ports, and explicit mounts | Development username and guest home path |
| Development user's sudo setting | Managed host home root |

Disk shrinking is rejected against the actual size reported by Lima, including
growth from an earlier failed update.

```{important}
A failed update does not undo every completed step.
The VM may already be stopped, Lima settings may have changed, or new environment files may be installed.
Read the error and follow [Troubleshooting](troubleshooting.md) before retrying.
`start` only starts the saved VM; it does not repair a failed update or apply corrected TOML.
```

## Storage and data

| Storage | Location and contents |
| -- | -- |
| Managed home | `<home.root>/<name>-<id>` on the Mac, mounted at `user.home` in the guest; user files, dotfiles, and home-based caches. |
| Project mounts | Mac directories listed in `[[mounts]]`; shared project files. |
| Guest disk | NixOS, the Nix store, and files outside host mounts, including service data under `/var` unless configured elsewhere. Only the applied NixOS generation is kept; see [The guest disk is full](troubleshooting.md#the-guest-disk-is-full). |
| Source inputs | Your TOML and module source directories on the Mac. |
| [Limanix state](troubleshooting.md#state-directories) | Ownership, operation records, prepared configurations, and imported modules. |

With the defaults, a managed home could be `~/.limanix/dev-box-a1b2c3d4e5f6`,
mounted at `/home/dev`. The suffix is generated; find the actual path in the
`create` output or the `home` field of `limanix list --json`.

```{important}
The managed home and read-write project mounts expose the same files on the Mac and in the guest.
Editing or deleting a file from either side changes the shared data.
A mount is not a backup.
```

| Operation | Guest disk | Managed home | Project mount sources |
| -- | -- | -- | -- |
| `stop` / `start` | Kept | Kept | Kept |
| `update` | Kept and reconfigured | Kept | Kept; mount settings can change |
| `delete`, with or without `--force` | Removed | Kept | Kept |
| `delete --remove-home`, with or without `--force` | Removed | Removed with all contents | Kept |

Retained storage can still be changed by guest software, including services
reconfigured by an update. To recreate an environment, keep the TOML and module
sources, back up home and project directories, and export any guest-only
application data before deleting the disk. The TOML does not contain your files
or database contents, and backing up the managed home does not preserve data
elsewhere on the guest disk. LimaNix has no VM snapshot or backup/restore
command.

## Delete a VM

```console
limanix delete dev-box
```

Deletion stops a running VM, removes its backend resources and saved VM record,
and records ownership of the preserved home. The command prints the home's path;
your TOML and project mount directories remain. Deletion also handles an already
missing backend.

To remove the managed home and all its contents too:

```console
limanix delete dev-box --remove-home
```

**Deletion has no confirmation prompt.** Save needed data before running it.

If ordinary deletion fails because Lima cannot stop the VM, request forced
backend deletion:

```console
limanix delete dev-box --force
```

`--force` does not imply `--remove-home`. Combine them only when you intend to
remove the home files too.

## Recreate a VM

After deletion, you can create another VM from the same TOML:

```console
limanix create --config limanix.toml
```

It receives a new identifier and a new empty managed home. The preserved home
stays at the path printed during deletion; LimaNix does not reattach it. To
reuse files, check the old and new paths, then copy the required data into the
new home or a project mount.

There is no CLI command to attach an archived home or restore a deleted guest
disk. After deleting the VM record, `delete NAME --remove-home` cannot remove
that archived home because the command needs a current managed identity. Keep
the printed path for later manual cleanup.
