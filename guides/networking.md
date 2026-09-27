# Networking

Connect from your Mac to the guest's IP address and the service's port.
The service must run, listen on a reachable guest interface, and be allowed through the guest firewall.

```{mermaid}
flowchart LR
    Mac["Mac application"] --> IP["Guest IP:8080"]
    IP --> Firewall["Guest firewall"]
    Firewall --> Service["Service on 0.0.0.0:8080"]
```

## Open a service port

For a service using TCP port `8080`, configure:

```toml
[network]
mode = "shared"

[network.ports]
tcp = [8080]
udp = []
```

`shared` is the only supported network mode.
Port numbers must be integers between `1` and `65535`.
Apply the change to an existing VM:

```console
limanix update --config ./limanix.toml
```

Start the application inside the guest and configure it to listen on `0.0.0.0:8080` or the guest's network address.
A listener bound only to `127.0.0.1:8080` accepts connections from inside the guest.

On your Mac, find the address:

```console
limanix list
```

Read the VM's `ADDRESS` column.
For an HTTP application, open `http://<guest-ip>:8080`, replacing `<guest-ip>` with that address.
The address is discovered from the running guest's shared-network IPv4 interface; it is not a configured static address.
Check the current value when reconnecting.

### Ports are firewall rules

| Setting | Effect |
| --- | --- |
| `tcp = [8080]` | Allow inbound TCP port `8080` through the guest firewall |
| `udp = [5353]` | Allow inbound UDP port `5353` through the guest firewall |
| `tcp = []` or `udp = []` | Add no openings from that configuration field |

These lists do not install or start services or map Mac ports to guest ports.
Limanix disables Lima's automatic application port forwarding: `localhost:8080` on the Mac is not an alias for the guest's port `8080`.
Base NixOS configuration and selected modules can add their own firewall rules, including when these lists are empty.

## Choose the guest architecture

The guest architecture and Mac hardware determine the backend:

| Mac hardware | `resources.arch` | Backend | Network |
| --- | --- | --- | --- |
| Apple Silicon | `arm64` | Apple Virtualization framework (VZ) | Native NAT |
| Intel | `amd64` | Apple Virtualization framework (VZ) | Native NAT |
| Apple Silicon | `amd64` | QEMU | Lima shared network using `socket_vmnet` |
| Intel | `arm64` | QEMU | Lima shared network using `socket_vmnet` |

Always run the client binary for your Mac's hardware.
VM operations reject the Intel client running under Rosetta on Apple Silicon.
For an Intel guest on Apple Silicon, use the `arm64` client with `resources.arch = "amd64"`.

The matching-architecture VZ path needs the native VZ driver included by the [client build](development.md#build-a-native-client).
It does not require QEMU or `socket_vmnet`.

### Prepare QEMU networking when needed

For a guest with a different architecture, install QEMU and make the guest's executable available in your Mac's `PATH`:

| Guest architecture | Executable |
| --- | --- |
| `amd64` | `qemu-system-x86_64` |
| `arm64` | `qemu-system-aarch64` |

Run as your regular Mac user in an interactive terminal:

```console
limanix network setup
```

The command checks the existing Lima network setup.
When installation is needed, it shows the helper and sudoers paths and asks for confirmation before invoking `sudo`.

| Setup item | Location |
| --- | --- |
| Bundled network helper | `/opt/socket_vmnet/bin/socket_vmnet` |
| Lima sudoers rules | `/private/etc/sudoers.d/lima` |

The helper is installed when missing; an existing helper must pass the setup checks.
An existing sudoers file is backed up before replacement.
Limanix and QEMU continue running as the regular user.

Creating or updating a QEMU guest can offer the same interactive installation.
A noninteractive command reports that setup is required instead of prompting.
Automatic installation supports Lima's standard helper, runtime, and sudoers paths.
For custom paths, have the administrator of that setup configure the helper and authorization.

## Check a connection in order

1. Run `limanix list` and check that the VM is running.
   `ADDRESS = -` means no guest address was reported; it does not identify the cause.
2. Run `limanix shell NAME` with your VM's name.
   Resolve a VM or SSH error before debugging the application.
3. Check that the application is running on the expected port and a reachable guest interface.
4. Check the protocol in the firewall configuration.
   An HTTP server normally needs its TCP port; opening the same number under UDP is a different rule.
5. After editing ports, run `limanix update --config ./limanix.toml`, wait for success, and check the address again.

[Virtual machines](virtual-machines.md) explains power commands and the difference between backend status and the last configuration operation.
