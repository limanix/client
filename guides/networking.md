# Networking

Connect from your Mac to the guest's IP address and the service's port. The
service must run, listen on a reachable guest interface, and be allowed through
the guest firewall. Its own access rules must also allow your connection.

```{mermaid}
flowchart LR
    Mac["Mac application"] --> IP["Guest IP:8080"]
    IP --> Firewall["Guest firewall"]
    Firewall --> Service["Service on 0.0.0.0:8080"]
```

## Configure service access

For each service you want to reach from the Mac, configure these parts:

| Part | Where to configure it | What to check |
| -- | -- | -- |
| Listener | The application's settings, startup arguments, or NixOS service options | Listen on the guest's network address or `0.0.0.0`, using the intended port |
| Guest firewall | `network.ports.tcp` or `network.ports.udp` in `limanix.toml` | Allow that port for the application's protocol |
| Application access | The service's authentication and authorization settings | Allow the connecting client and configure the account, password, or other credentials it requires |

The port lists configure only the guest firewall. They do not change listeners,
restrict source addresses, create accounts, or grant application permissions.
Listening on `0.0.0.0` means all IPv4 interfaces in the guest; it does not
restrict access to the Mac. If the service has rules for client addresses, use
the Mac's source address as observed by that service, not the guest's `ADDRESS`
from `limanix list`.

The listener and access settings differ between services. For PostgreSQL, follow
[Connect from the Mac](https://limanix.dev/categories/nixos/modules/postgres/README.html#connect-from-the-mac)
for `listen_addresses`, client authentication rules, and a connection example.
The steps below cover the shared LimaNix configuration.

## Open a service port

For a service using TCP port `8080`, edit the existing tables in `limanix.toml`
on the **Mac**:

```toml
[network]
mode = "shared"

[network.ports]
tcp = [8080]
udp = []
```

`shared` is the only supported network mode. Port numbers must be integers
between `1` and `65535`. Keep any existing ports and add `8080` to the TCP list;
do not duplicate the TOML tables. Apply the change to an existing VM:

```console
limanix update --config ./limanix.toml
```

The update restarts the VM. After it succeeds, open a guest shell with
`limanix shell NAME`, replacing `NAME` with your VM's name. Start a manually
managed application again, or check that its configured system service has
started. Configure it to listen on `0.0.0.0:8080` or the guest's network
address, allow your client in its access rules, and restart or reload it as
required by that application. A listener bound only to `127.0.0.1:8080` accepts
connections from inside the guest.

On your Mac, find the address:

```console
limanix list
```

Read the VM's `ADDRESS` column. For an HTTP application, open
`http://<guest-ip>:8080`, replacing `<guest-ip>` with that address. The guest
owner `lmx` reports it from the running guest's shared-network IPv4 interface;
it is not a configured static address. Check the current value when
reconnecting.

### Try an HTTP connection

To check this path without your own application, serve a temporary file with the
Python module. On your **Mac**, run `limanix modules list` and confirm that
`lmx:python` is available in your client. Edit the existing `[nixos]` and
`[network.ports]` tables in `limanix.toml` so their lists include the module and
port:

```toml
[nixos]
modules = ["lmx:python"]

[network.ports]
tcp = [8080]
```

These are the relevant parts of the file; do not add a second copy of either
table. If either list already has entries, keep them and add `lmx:python` or
`8080` only if missing. Apply both changes with
`limanix update --config ./limanix.toml`.

Open the guest with `limanix shell NAME`, replacing `NAME` with your VM's name.
Inside the **VM**, run:

```console
mkdir -p /tmp/limanix-http-demo
printf 'Hello from Limanix\n' > /tmp/limanix-http-demo/index.html
cd /tmp/limanix-http-demo
python -m http.server --bind 0.0.0.0 8080
```

Keep that shell open. In another terminal on your **Mac**, run `limanix list`
and open `http://<guest-ip>:8080/` in a browser, replacing `<guest-ip>` with the
VM's `ADDRESS`. The page should show `Hello from Limanix`. Return to the guest
shell and press Ctrl+C to stop the server.

### Ports are firewall rules

| Setting | Effect |
| -- | -- |
| `tcp = [8080]` | Allow inbound TCP port `8080` through the guest firewall |
| `udp = [5353]` | Allow inbound UDP port `5353` through the guest firewall |
| `tcp = []` or `udp = []` | Add no openings from that configuration field |

These lists do not install or start services or map Mac ports to guest ports.
LimaNix disables Lima's automatic application port forwarding: `localhost:8080`
on the Mac is not an alias for the guest's port `8080`. Base NixOS configuration
and selected modules can add their own firewall rules, including when these
lists are empty.

## Choose the guest architecture

The guest architecture and Mac hardware determine the backend:

| Mac hardware | `resources.arch` | Backend | Network |
| -- | -- | -- | -- |
| Apple Silicon | `arm64` | Apple Virtualization framework (VZ) | Native NAT |
| Intel | `amd64` | Apple Virtualization framework (VZ) | Native NAT |
| Apple Silicon | `amd64` | QEMU | Lima shared network using `socket_vmnet` |
| Intel | `arm64` | QEMU | Lima shared network using `socket_vmnet` |

Always run the client binary for your Mac's hardware. VM operations reject the
Intel client running under Rosetta on Apple Silicon. For an Intel guest on Apple
Silicon, use the `arm64` client with `resources.arch = "amd64"`.

The matching-architecture VZ path needs the native VZ driver included by the
[client build](development.md#build-a-native-client). It does not require QEMU
or `socket_vmnet`.

### Prepare QEMU networking when needed

For a guest with a different architecture, install QEMU and make the guest's
executable available in your Mac's `PATH`:

| Guest architecture | Executable |
| -- | -- |
| `amd64` | `qemu-system-x86_64` |
| `arm64` | `qemu-system-aarch64` |

Run as your regular Mac user in an interactive terminal:

```console
limanix network setup
```

The command checks the existing Lima network setup. When installation is needed,
it shows the helper and sudoers paths and asks for confirmation before invoking
`sudo`.

| Setup item | Location |
| -- | -- |
| Bundled network helper | `/opt/socket_vmnet/bin/socket_vmnet` |
| Lima sudoers rules | `/private/etc/sudoers.d/lima` |

The helper is installed when missing; an existing helper must pass the setup
checks. An existing sudoers file is backed up before replacement. LimaNix and
QEMU continue running as the regular user.

Creating or updating a QEMU guest can offer the same interactive installation. A
noninteractive command reports that setup is required instead of prompting.
Automatic installation supports Lima's standard helper, runtime, and sudoers
paths. For custom paths, have the administrator of that setup configure the
helper and authorization.

## Check a connection in order

On your **Mac**, check the service's port, replacing `dev-box` and `8080` with
your VM's name and the port:

```console
limanix network check dev-box 8080
```

The command runs these checks in order and prints a hint under a check that
needs attention:

| Check | What it checks |
| -- | -- |
| `vm` | The VM is running, and its last create or update completed. |
| `address` | The guest has an IPv4 address on the shared network. |
| `firewall` | The guest firewall opens the port for its protocol. |
| `listener` | Something listens on the port on an address other than loopback. |
| `process` | Which process and user hold the socket, when something listens. |
| `connect` | The Mac connects to the address and port within 3 seconds. |

Add `--udp` for a UDP port; UDP has no connection to try, so `connect` is left
out. `--json` prints the same checks for scripts. A failed check makes the
command exit with status 1.

When the guest checks pass but `connect` fails, the connection stops on the Mac:
check a VPN, a firewall, and the Local Network permission of your terminal app
in System Settings › Privacy & Security.

To check by hand, or when every check passes but your client still fails:

1. Run `limanix list` and check that the VM is running. `ADDRESS = -` means no
   guest address was reported; it does not identify the cause. A notice under
   the row says what to do first.
1. Run `limanix shell NAME` with your VM's name. Resolve a VM or SSH error
   before debugging the application.
1. Check that the application is running on the expected port and a reachable
   guest interface.
1. Check the protocol in the firewall configuration. An HTTP server normally
   needs its TCP port; opening the same number under UDP is a different rule.
1. After editing ports, run `limanix update --config ./limanix.toml`, wait for
   success, and check the address again. Restart a manually managed application
   after the VM restarts.
1. If the service responds with an authentication or permission error, check its
   client access rules and the credentials you supplied. Opening another
   firewall port does not resolve an application access denial.

A TCP refusal or timeout alone does not identify the failing setting. Check the
address, listener, and firewall before changing application credentials.

[Virtual machines](virtual-machines.md) explains power commands and the
difference between backend status and the last configuration operation.
