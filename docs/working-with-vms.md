# Work with a VM

Use `create` once, `update` when the configuration changes, and `start` / `stop` for everyday power control.
Run the Limanix commands on your **Mac**.
The examples use a VM named `dev-box`; replace it with your own name.

## Choose the right operation

| You want to… | Run on your Mac | What happens |
| --- | --- | --- |
| Create a VM from a file | `limanix create --config limanix.toml` | Allocate a VM and managed home, build its NixOS configuration, restart, and check the development account. |
| Apply configuration or module changes | `limanix update --config limanix.toml` | Keep the VM identity and storage, apply a new configuration, and restart. |
| Resume an existing VM | `limanix start dev-box` | Start the saved VM without rereading your TOML file or rebuilding NixOS. |
| Pause work | `limanix stop dev-box` | Shut down the VM; keep its disk and managed home. |
| Open a guest terminal | `limanix shell dev-box` | Connect as the configured development user and enter that user's home. |
| Check saved VMs | `limanix list` | Show backend power state, Limanix operation state, and the discovered guest address. |
| Remove a VM | `limanix delete dev-box` | Delete the VM disk and saved VM configuration; preserve its managed home. |

`name` inside the TOML file selects the VM for `create` and `update`.
The other commands take that name directly.
Keep your configuration file: changing it does not affect a running VM until you apply an update.

## Enter the guest

Open an interactive terminal from your **Mac**:

```console
limanix shell dev-box
```

Inside the **VM**, check where you are:

```console
whoami
pwd
```

With the default configuration, these report `dev` and `/home/dev`.
If you mounted a project at `/workspace`, use `cd /workspace` to work on it.
Type `exit` to return to your Mac. Leaving the shell keeps the VM running.

For a single guest command, keep the terminal on your Mac:

```console
limanix shell dev-box -- id
limanix shell dev-box -- pwd
```

The command runs as the development user and starts in their home.
Its exit status is returned to your host shell.

### Shell expressions need a guest shell

Limanix passes command arguments through; it does not interpret a command string for you.
Use a guest shell when you need `cd`, a pipeline, or guest-side variable expansion:

```console
limanix shell dev-box -- bash -lc 'cd /workspace && pwd'
limanix shell dev-box -- bash -lc 'printf "%s\n" "$HOME"'
```

The single quotes keep your Mac's shell from expanding `$HOME` before the command reaches the VM.

## Apply a configuration change

1. Save running work. An update interrupts guest sessions and services.
2. Edit the existing TOML file. See [Configuration](configuration.md) and [Modules](modules.md).
3. Apply it from your Mac:

   ```console
   limanix update --config limanix.toml
   ```

4. After the command succeeds, reconnect and check the tool or service you changed.

```{mermaid}
flowchart TD
    A["Check configuration and prepare inputs"] --> B["Stop VM and apply Lima settings"]
    B --> C["Start guest and build NixOS"]
    C --> D["Restart, check user, record ready"]
```

The NixOS build prepares the system for the next boot.
Limanix restarts the guest after a successful build and checks that the configured development user can run a command.
A stopped VM is also started by `update`.

| Can change through `update` | Requires a new VM |
| --- | --- |
| CPU, memory, and disk growth | Guest architecture |
| Selected modules, environment, firewall ports, and explicit mounts | Development username and guest home path |
| Development user's sudo setting | Managed host home root |

Disk shrinking is rejected.
Limanix checks the actual disk size reported by Lima, including growth that may already have happened during a failed update.
Changing `name` selects a different VM; it does not rename the existing one.

```{important}
A failed update does not undo every completed step.
The VM may already have stopped, its Lima configuration may have changed, or new environment files may have been installed.
Read the error, inspect `limanix list`, and follow [Troubleshooting](troubleshooting.md) before retrying.
```

Installing another client binary does not update existing guests automatically.
Run `update` to apply that client's bundled configuration and selected modules.

## Read both status columns

```console
limanix list
limanix list --json
```

`STATUS` describes the **Lima backend**.
`STATE` describes the **last Limanix operation**.
They answer different questions:

| Example | Meaning |
| --- | --- |
| `Running` / `ready` | The backend is running; the last create or update completed. |
| `Stopped` / `ready` | The configuration was applied successfully; the VM is now stopped. |
| `Running` / `error` | The backend is running, but a Limanix operation failed. Read the accompanying diagnostic. |
| Any status / `interrupted` | A create, update, or delete record remained after its operation lock was released. |
| `Missing` | Limanix could not match a backend instance to the saved identity, or that identity could not be read. |
| Any status / `corrupt` | Limanix could not read a valid operation record. The diagnostic identifies the problem. |

During active work, `STATE` can be `creating`, `updating`, or `deleting`.
Starting or stopping a VM does not reset a previous operation error to `ready`.

**`Running` alone does not prove that the guest is ready.**
Even `ready` is a saved result, not a continuous application health check.
To check access now:

```console
limanix shell dev-box -- true
```

A successful exit checks guest access as your development user.
Check your application's own endpoint or command separately.
An address shown as `-` means that no shared-network IPv4 address was discovered; it does not by itself mean the VM failed.
See [Networking](networking.md) for service access.

## Stop now, continue later

On your **Mac**:

```console
limanix stop dev-box
limanix start dev-box
limanix shell dev-box
```

`start` uses the already saved VM configuration.
It does not repair a failed NixOS update or pick up new TOML edits.
Use [Storage and recovery](storage-and-recovery.md) before deleting or replacing a VM.
