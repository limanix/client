# Storage and recovery

Your VM uses several kinds of storage.
**Deleting the VM preserves its managed home by default, but removes its guest disk.**
Know where important files live before choosing a cleanup command.

## Where files live

| Storage | Location | What belongs there |
| --- | --- | --- |
| Managed home | On your Mac at `<home.root>/<name>-<id>`, mounted at `user.home` in the guest | The development user's files, dotfiles, and home-based caches. |
| Explicit project mounts | The Mac directories in `[[mounts]]` | Existing project files shared with the guest. |
| Guest disk | The Lima instance | NixOS, the Nix store, and guest-only files outside host mounts. Service data under `/var` normally lives here unless a service uses another path. |
| Limanix state | `~/Library/Application Support/Limanix` by default on macOS | VM ownership, operation records, prepared configurations, and imported modules. |
| Your TOML and module source | Wherever you keep them on your Mac | Inputs you edit and use to create or update VMs. |

With the defaults, a home could be `~/.limanix/dev-box-a1b2c3d4e5f6`, mounted at `/home/dev`.
The suffix is a generated identifier, not a value to copy from this example.
Find the actual path in the output of `create`, or in the `home` field of:

```console
limanix list --json
```

```{important}
A read-write mount exposes the same files to the Mac and guest.
Editing or deleting a file from either side changes the shared data.
The managed home is also read-write; it is not a backup copy of a guest directory.
```

## What each operation preserves

| Operation | Guest disk | Managed home | Explicit host mount sources |
| --- | --- | --- | --- |
| `stop` / `start` | Kept | Kept | Kept |
| `update` | Kept and reconfigured | Kept | Source directories kept; mount configuration can change |
| `delete` | Removed | Kept | Kept |
| `delete --force` | Removed | Kept | Kept |
| `delete --remove-home` | Removed | Removed, including its contents | Kept |
| `delete --force --remove-home` | Removed | Removed, including its contents | Kept |

“Kept” means the operation retains that storage; software running in the guest can still write to it.
For example, changing a module can change what a service does with its existing data.

## Delete deliberately

### Keep the managed home

Run on your **Mac**:

```console
limanix delete dev-box
```

Limanix stops a running VM, removes its backend resources, records the preserved home's ownership, and removes the saved VM record.
The command prints the preserved home path.
Your source TOML file and explicit host mount directories remain.

### Also remove the managed home

```console
limanix delete dev-box --remove-home
```

This deletes the VM disk **and every file in its recorded managed home**.
The CLI has no confirmation prompt.
Save anything you need before running it.

### Force deletion after a stop failure

If ordinary deletion fails because Lima cannot stop the VM, `--force` requests forced backend deletion:

```console
limanix delete dev-box --force
```

`--force` does not imply `--remove-home`.
Only add the latter when you also intend to delete the home files.

## Recreating the same name creates a new home

After deletion, you can use the same TOML file to create another VM:

```console
limanix create --config limanix.toml
```

The new VM receives a new identifier and a new empty managed home.
Limanix does not automatically adopt or reattach a home preserved from the previous VM.
The old directory remains at the path printed during deletion.

To reuse files, copy the data you need from that preserved directory into the new managed home or a project mount.
Check the old and new paths first.
There is no CLI command for attaching an archived home or restoring a deleted guest disk.

Once the VM record has been deleted, running `delete NAME --remove-home` cannot remove its archived home: the command needs a current managed identity.
Retain the printed path for any later manual archive cleanup.

## Keep recoverable inputs and data

For a VM you need to recreate, keep these separately:

- **Configuration:** the TOML file and source directories for your own modules.
- **Home and projects:** backups of the relevant Mac directories.
- **Guest-only application data:** an export or backup appropriate for that application, before deleting its disk.

The TOML file describes the environment; it does not contain your files or database contents.
Saving only the managed home does not preserve data elsewhere on the guest disk.
Limanix does not provide a VM snapshot or backup/restore command.

## State directories are not configuration files

Limanix keeps separate ownership and operation records:

```text
Limanix/
├── instances/<name>/
│   ├── identity.json
│   ├── instance.json
│   └── generations/<id>/
│       ├── lima.yaml
│       ├── environment
│       ├── environment.sh
│       └── flake/
├── homes/
├── modules/
├── locks/
└── runtime/
```

`identity.json` records the VM's exact backend identity and owned home.
`instance.json` records the selected input generation and operation result.
The `homes` directory holds ownership records for preserved homes; the home files stay at their original host paths.

Prepared generations are generated inputs, not user-maintained configuration or a rollback history.
A successful update removes old input generations.
Change the source TOML or module files and apply an update instead of editing generated files.

`LIMANIX_HOME` overrides the Limanix state root.
Lima stores backend instances separately under `~/.lima`, or `LIMA_HOME` when set.
Neither override moves existing VMs or home directories.
Use the same environment when operating existing VMs; pointing the CLI at another root changes the state it can see.

## Recover a failed operation

Start with [Troubleshooting](troubleshooting.md) and the original error.
If the backend still exists and both saved records are valid, correcting the input and retrying `update` can complete a failed create or update.
Deletion can also clean up an identity whose backend is already missing.

Do not remove saved ownership files or change them to make an error disappear.
Limanix uses them to identify the exact VM and managed home it is allowed to operate on.
If ownership itself cannot be read, preserve the files and investigate that diagnostic before removing anything.
