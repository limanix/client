# Configuration

Keep a `limanix.toml` beside your project. It describes the VM's resources,
guest account, shared directories, environment, firewall ports, and selected
NixOS modules.

The **Mac** is the host; the **Linux VM** is the guest. A path on one side does
not automatically exist on the other.

| Related guide | Configure |
| -- | -- |
| [Modules](modules.md) | Tools and services from the catalog or your own NixOS modules |
| [Networking](networking.md) | Guest service access, firewall ports, and QEMU networking |

## Start with a complete configuration

Save this example as `limanix.toml` in an existing project directory. It shares
that directory with the guest at `/workspace` and creates a separate persistent
home for the `dev` account.

```toml
# Save this file in your project directory as limanix.toml.
# Apple Silicon: arm64. Intel Mac: change resources.arch to amd64.
schema_version = 1
name = "project-box"

[resources]
arch = "arm64"
cpu = 4
mem = "8GiB"
disk = "10GiB"

[user]
name = "dev"
home = "/home/dev"
sudo = true

[home]
root = "~/.limanix"

[nixos]
modules = []

[network]
mode = "shared"

[network.ports]
tcp = [8080]
udp = []

[env]
APP_ENV = "development"

[[mounts]]
source = "."
target = "/workspace"
mode = "rw"
```

[Open or download the project configuration](examples/project.toml).

The example targets **Apple Silicon**. On an **Intel Mac**, set
`resources.arch = "amd64"`. For a guest with a different architecture from your
Mac, see [Networking](networking.md).

With the client installed and the host prerequisites met:

```console
limanix create --config ./limanix.toml
limanix shell project-box
```

Inside the guest:

```console
cd /workspace
```

This creates a base NixOS VM: `modules = []` adds no optional tools, and opening
TCP port `8080` does not start an application. Select tools in
[Modules](modules.md), then apply the edited configuration with
`limanix update --config ./limanix.toml`.

## Read the TOML structure

| In the example | Meaning |
| -- | -- |
| `schema_version` and `name` before the first table | Settings at the top level |
| `[resources]` | A table grouping the guest architecture, CPU, memory, and disk |
| `[network.ports]` | A table inside `network` for TCP and UDP ports |
| `[[mounts]]` | One entry in the list of shared directories; repeat the block for each directory |
| `tcp = [8080]` and `udp = []` | Lists of values inside square brackets; an empty list selects none |

A table continues until the next table header. To disable explicit mounts, put
`mounts = []` **before the first table**, then remove every `[[mounts]]` block.

Values have types: `cpu = 4` is an integer, `sudo = true` is a boolean, and
`mem = "8GiB"` is a string. Unknown fields and wrong types are rejected,
including typos such as `resources.cpus`.

## Choose resources and identity

| Setting | What to choose |
| -- | -- |
| `name` | A local VM name: 1–63 lowercase letters, digits, or hyphens; start and end with a letter or digit. |
| `resources.arch` | `arm64` for Apple Silicon or `amd64` for Intel. These spellings differ from Nix's `aarch64` and `x86_64`. |
| `resources.cpu` | A positive whole number of virtual CPUs. |
| `resources.mem` | A positive whole number of GiB, written as a quoted string such as `"8GiB"`. |
| `resources.disk` | The guest system disk size, also in whole GiB. Shared host directories are separate from this disk. The size also limits the number of files: about one inode per 16 KiB. |
| `schema_version` | Keep `1`; this is the configuration contract version, independent of the client release. |

Sizes such as `"8GB"`, `"1.5GiB"`, `"0GiB"`, and `"08GiB"` are rejected.

`name` identifies the VM used by `update`. Changing it in the file does not
rename an existing VM.

## Keep the three home paths distinct

| Setting or directory | Side | Purpose |
| -- | -- | -- |
| `home.root = "~/.limanix"` | Mac | Parent directory for managed guest homes. |
| `<home.root>/<name>-<id>` | Mac | The particular home allocated to this VM. Limanix creates it. |
| `user.home = "/home/dev"` | Guest | Where that managed directory appears inside Linux. |

The Mac's `~` belongs to the account running LimaNix, not the guest user. The
Mac filesystem root `/` cannot be used as `home.root`.

`user.name` creates the regular account used by `limanix shell`. The guest
account uses the Mac account's numeric UID for shared-file ownership; its name
can be different from the Mac account's name. LimaNix creates a guest group for
that account.

Guest usernames start with a lowercase letter or `_`, followed by lowercase
letters, digits, `_`, or `-`, up to 32 characters. `root` and `limanix-admin`
are reserved.

`user.sudo = true` gives this account passwordless sudo **inside the guest**.
Set it to `false` to omit that permission. This setting does not grant macOS
administrator rights.

[Virtual machines](virtual-machines.md#storage-and-data) explains which files
survive VM deletion.

## Share project directories

Each `[[mounts]]` maps an existing Mac directory to a guest path:

```toml
[[mounts]]
source = "."
target = "/workspace"
mode = "rw"

[[mounts]]
source = "../fixtures"
target = "/mnt/fixtures"
mode = "ro"
```

Create the sibling directory `../fixtures` on the Mac before using the second
mount.

- `rw` lets guest processes modify the Mac directory. It is the default when
  `mode` is omitted.
- `ro` makes this mount read-only inside the guest.
- `source` and `target` are required for every entry.
- `mounts = []` disables explicit mounts. The managed guest home remains
  mounted.

### Resolve paths from the configuration

If your file is `/Users/alex/work/api/limanix.toml`:

| Host path in the file | Resolved path |
| -- | -- |
| `"."` | `/Users/alex/work/api` |
| `"./fixtures"` | `/Users/alex/work/api/fixtures` |
| `"../shared"` | `/Users/alex/work/shared` |
| `"~/projects"` | `projects` in the current Mac user's home |

Relative `mounts.source` and `home.root` values are resolved from the
configuration file's directory, regardless of the directory where you run the
command. If the configuration is a symlink, the real file's location is used.
Existing symlinks within host paths are also resolved.

Only a leading `~` is expanded. `$HOME` and `${PROJECT}` stay literal; they do
not read shell environment variables.

Mount sources must be directories that already exist when you create or update
the VM. LimaNix creates its managed home separately.

### Choose guest destinations

Guest paths must be absolute and cannot be `/`. They cannot contain whitespace
or `:`, and cannot include `..` path components. Paths must start with a single
`/`; repeated slashes elsewhere and `.` components are normalized.

Explicit mounts cannot overlap each other: `/workspace` and `/workspace/cache`
are rejected as two separate mount targets. A mount also cannot replace the
managed guest home or cover it from a parent directory.

A mount **inside** the managed home is allowed, for example
`/home/dev/.config/nvim` when `user.home = "/home/dev"`.

Limanix reserves these destinations and their children for the guest system:

```text
/etc   /boot  /usr  /var   /nix  /run  /dev
/proc  /sys   /bin  /sbin  /mnt/limanix  /home/limanix-admin
```

`/home` is also unavailable: it would cover the management account's home. The
same destination rules apply to `user.home`.

## Set the guest environment

Use `[env]` for values needed by guest login sessions and system or user
services:

```toml
[env]
APP_ENV = "development"
APP_PORT = "8080"
OPTIONAL_SETTING = ""
```

Every value must be a string, including numbers. An empty string is valid. Names
use ASCII letters, digits, and `_`, and cannot start with a digit.

Values are literal: `TOKEN = "$TOKEN"` gives the guest the text `$TOKEN`. It
does not copy the Mac's `TOKEN` variable or execute a shell command.

```{important}
`[env]` is guest-wide plaintext configuration.
Values are written to runtime files and copied into guest-readable files under `/etc/limanix`.
They are kept outside the Nix store, but this is not encrypted secret storage.
```

## Understand omitted values

Omitting a setting keeps its model default. **The generated defaults contain
example paths and application settings.** Use explicit empty lists and tables to
remove unwanted defaults:

| Your file | Effective behavior |
| -- | -- |
| No `mounts` setting | Keep the two example mounts: `~/projects/my-project` → `/workspace` and `~/.ssh/limanix` → `/mnt/git-keys`. These source directories must exist. |
| `mounts = []` | No explicit mounts. |
| One or more `[[mounts]]` entries | Use those entries instead of the example mounts. |
| No `[env]` table | Keep `APP_ENV = "development"` and `APP_LOG_LEVEL = "debug"`. |
| An empty `[env]` table | Set no Limanix environment variables. |
| A nonempty `[env]` table | Use exactly those entries; example environment keys are not added. |
| No `network.ports.tcp` setting | Keep TCP port `8080`. |
| `tcp = []` | Add no TCP openings from this field. Base configuration and modules can declare their own firewall rules. |
| `modules = []` | Add no optional NixOS modules; retain the guest base. |
| `[resources]` with only `cpu = 2` | Change CPU count; keep the default architecture, memory, and disk values. |

An empty string does not request a default. For example, `source = ""` is an
error; `APP_ENV = ""` is a literal empty environment value.

## Apply changes deliberately

Editing the file alone leaves the VM unchanged. Apply it with:

```console
limanix update --config ./limanix.toml
```

The update retains the VM and its managed home, applies the new configuration,
and restarts the guest. Architecture, guest username, guest home path, and host
home root are fixed when the VM is created. Disk size can grow but cannot
shrink.

[Reference](reference.md) lists every field and its default.
[Networking](networking.md) covers ports and connectivity.

```{toctree}
:hidden:

modules
networking
```
