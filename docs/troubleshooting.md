# Troubleshooting

Start with the command that failed and its original output.
Then inspect the saved and live state from your **Mac**:

```console
limanix --version
limanix list
limanix list --json
```

`list` does not repair or restart a VM.
Keep the VM name, both status fields, and the error text together when investigating a failure.

## Creation fails before the VM starts

| Diagnostic | What to do |
| --- | --- |
| `required flag --config was not provided` | Pass the file explicitly: `limanix create --config limanix.toml`. |
| Invalid TOML, unsupported schema, or unknown field | Fix the field identified in the error. See [Configuration](configuration.md). |
| A host mount directory cannot be found | Create the intended source directory or correct its path. Relative sources are resolved against the TOML file's directory. |
| `set home.root to a writable directory` | Choose a host directory your normal Mac account can write to. |
| `run Limanix as your regular host user, not root` | Run `create` or `update` without `sudo`. |
| `VM '…' already has state; use update or delete` | Inspect `list`. Use `update` to retry an existing VM; use `delete` only when you intend to remove it. |
| A module cannot be found | Compare the selection with `limanix modules list`. Import local modules before selecting them. See [Modules](modules.md). |
| `SSH socket path needs … bytes` | Use a shorter VM name or a shorter Lima storage root. Changing a root does not relocate existing instances. |

`first-config` writes an editable example, including example mount paths.
Check those paths before creating the VM.
Do not run `first-config` over a configuration you need to preserve: it overwrites an existing file.

For macOS version, native binary, SSH, QEMU, or shared-network setup errors, return to [Installation](installation.md) and [Networking](networking.md).

## `Running` appears, but the shell fails

`Running` is the backend power state.
It does not confirm that a NixOS rebuild finished or that the development account is accessible.

1. Check `STATE` and the diagnostic below that VM in `limanix list`.
2. If `create` or `update` is still running, let it finish.
3. If the VM is stopped, run `limanix start NAME`.
4. Check guest access with `limanix shell NAME -- true`.

Replace `NAME` with the VM's actual name.
If access still fails, keep the SSH or guest error rather than treating the power state as success.
After a failed configuration operation, follow the next section.

## Create or update failed during provisioning

A failed operation may leave a valid VM and managed home for recovery.
It does not necessarily leave the guest exactly as it was before the command.

| Failure point | What may have happened |
| --- | --- |
| Input validation or preparation | Guest application has not started. A rejected update can leave the previous operation state unchanged. |
| Stopping or editing the backend | The VM may already be stopped or have new Lima settings. |
| NixOS evaluation or build | New environment files have already been installed. The failed build does not trigger Limanix's post-build restart. |
| Restart or development-user check | The NixOS build may have succeeded, but the complete operation has not been marked ready. |

For a recoverable VM:

1. Fix the reported problem in the TOML file or module source.
2. If you changed an imported module, replace its registry copy as described in [Modules](modules.md).
3. Retry on your **Mac**, using a file with the same VM name:

   ```console
   limanix update --config limanix.toml
   ```

4. Check the command result and `limanix list` again.

Retrying `create` for a name with saved state is rejected.
Use `update` when the existing backend and records are usable.
If the backend is missing, use the missing-backend path below.

**A successful `start` is not a successful update.**
It starts the saved VM without applying your corrected TOML file or resetting an operation error.

## An update is rejected

| Diagnostic | Meaning and next step |
| --- | --- |
| `an update cannot change architecture, username, or managed-home paths` | These are fixed at creation. Revert that change or create a different VM and migrate the files you need. |
| `shrinking the guest disk is not supported` | Set the requested disk to at least the actual allocated size. A failed earlier update may already have enlarged it. |
| `lima did not report the disk size; update was not started` | Limanix cannot validate disk safety. Inspect the backend diagnostic before attempting another update. |
| `another operation is running for VM` | Another process holds that VM's operation lock. Let it complete or cancel it from its original terminal. |

The immutable home settings are `home.root` and `user.home`.
See [Work with a VM](working-with-vms.md) for the settings that can change in place.

## An operation was interrupted

An abandoned `creating`, `updating`, or `deleting` record appears as `interrupted` when no process holds its operation lock.
This is a listing result; the command does not rewrite the saved record or roll back the guest.

- After an interrupted create or update, inspect the original output, fix the issue, and retry `update` if the backend exists.
- After an interrupted deletion, retry the intended `delete` command. Check whether it included `--remove-home` before repeating it.
- If the guest rebuild cancellation reports `cannot confirm guest rebuild stopped`, do not assume all guest build work has ended. Keep the service name included in that error for investigation.

Limanix uses operating-system file locks, released when their owning process exits.
The presence of a `.lock` file does not mean an operation is still running.
Do not remove lock files to bypass a live lock.

`Waiting for another Limanix network lifecycle operation` is different: starts, stops, and deletions sharing the same Lima root serialize their network changes.
The waiting command continues after the other operation releases that lock, or exits if you cancel it.

## The backend is missing

If the diagnostic says `lima instance for '…' is missing`, first check that you are using the same `LIMA_HOME` and `LIMANIX_HOME` values as before.
A changed storage root can make saved records and backend instances appear disconnected.

If the backend was actually removed and you intend to discard its saved Limanix record:

```console
limanix delete dev-box
```

Deletion handles a missing backend and preserves the managed home by default.
It does not reconstruct the missing guest disk.
Read [Storage and recovery](storage-and-recovery.md) before recreating the VM.

## A saved record is corrupt

Limanix reads VM ownership separately from the mutable operation record.
When `instance.json` is damaged but `identity.json` is still valid, listing can still show the backend and home; start, stop, and delete use that identity independently.
`update` needs a valid operation record and cannot repair arbitrary corrupt JSON.

Deletion is an available cleanup path with valid ownership, but it still destroys the VM disk.
Preserve needed data first.
If `identity.json` is also unreadable, do not invent an identity or remove its checks: the CLI cannot safely establish which backend and home belong to that record.

## The address or application is unavailable

An `ADDRESS` of `-` means the guest's shared-network IPv4 address was not discovered.
Stopped VMs, a failed SSH probe, or an interface without that address all produce an empty result.
Check guest access first:

```console
limanix shell dev-box -- true
```

If the shell works, continue with [Networking](networking.md) to check the service's listening address and guest firewall ports.
A successful shell connection does not confirm that a separate application is healthy.

## Keep useful diagnostics

The failed command's terminal output is the main source for Nix build and guest command errors.
The saved operation error contains a recovery message and state-file path; it does not retain the complete build log.

For backend startup diagnostics, take `lima_name` from `limanix list --json`.
That instance's directory is under `~/.lima`, or your `LIMA_HOME` override.
Lima writes host-agent output to `ha.stdout.log` and `ha.stderr.log` there.

When reporting a problem, include:

- The client version and exact command.
- The original error and relevant surrounding output.
- The VM's JSON listing and the relevant configuration fields.
- The host architecture and whether the failure happened during create, update, start, or shell access.

Review logs and configuration before sharing them: environment values, paths, and guest-produced output can contain private data.
