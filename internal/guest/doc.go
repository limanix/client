// Package guest provisions NixOS guests and opens development-user sessions.
//
// [Guest] uses a [Client] management connection supplied by internal/lima. It does not allocate host homes,
// persist VM status, or construct flake inputs; those responsibilities belong to internal/vm, internal/state,
// and internal/nixos.
//
// # Applying a generation
//
//	read-only /mnt/limanix inputs
//	             ↓
//	install environment files into /etc/limanix
//	             ↓
//	nixos-rebuild boot → stop → start → verify development-user session
//
// [Guest.Apply] installs runtime ENV before rebuilding. A failed rebuild returns without rebooting; environment
// installation may already have happened. Successful rebuilds restart through the client and check that the regular
// development user can execute a command.
// Rebuilds run in a transient guest systemd unit. Cancellation stops that unit before returning; it does not stop
// the VM or roll back changes already applied. Failure to confirm the stop is reported as an error.
// When installing environment files or rebuilding fails, Apply reads the usage of the store file system and adds a
// [DiskError] if inodes or bytes are nearly exhausted or the guest reported ENOSPC. ext4 fixes its inode count when
// the disk is sized, so a store of small files can fill it while bytes remain free.
//
// # Pruning the guest
//
//	delete system generations except the booted one
//	             ↓
//	rewrite boot entries for the remaining generation
//	             ↓
//	collect store paths that nothing references
//
// [Guest.Reserve] runs before an apply and before an update stops a running guest. It collects unreferenced store
// paths when free bytes or inodes fall below [domain.DiskCollectPercent] and returns a [DiskError] below
// [domain.DiskMinimumPercent]; Apply continues either way. [Guest.Disk] reads the same usage for listings.
//
// [Guest.Prune] runs after an apply has been recorded as ready. LimaNix boots only the generation it applied; older
// ones, including the base image's, keep their closures and kernel copies in /boot alive. Collection also removes
// evaluation-time sources such as pinned Nixpkgs trees. Generations stay when their removal fails, and the store is
// still collected.
//
// # Sessions and address discovery
//
// [Guest.Shell] changes to the development user's home and preserves literal command arguments and exit status.
// An empty argument list opens a login shell. The management account and the development account are distinct.
//
// [Guest.Address] probes running guests only when a shared-network MAC is known. It matches that MAC in ip -j address
// output and selects a global IPv4 address. An individual probe timeout, SSH failure, or unavailable address yields
// an empty string. The listing layer propagates cancellation of the parent operation. The client must support
// concurrent address probes.
//
// Read apply.go for provisioning order, rebuild.go for cancellation, disk.go for full-disk diagnostics, prune.go for
// generation removal, shell.go for user switching, and address.go for best-effort network discovery.
package guest
