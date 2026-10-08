// Package guest applies generations through lmx, the guest owner, and opens development-user sessions.
//
// [Guest] uses a [Client] management connection supplied by internal/lima. It does not allocate host homes,
// persist VM status, or construct flake inputs; those responsibilities belong to internal/vm, internal/state,
// and internal/nixos. It reads version 1 of the lmx host contract: a command answers with one JSON envelope, and
// the answer wins over the exit status. Copies of the pinned release's contract examples in testdata keep the
// decoders honest.
//
// # Applying a generation
//
//	lmx store reserve → stop the guest's lmx units
//	             ↓
//	nix build packages.<system>.lmx of the mounted generation → /run/limanix-lmx
//	             ↓
//	run its lmxd as lmx-transient.service → lmx apply --follow
//	             ↓
//	stop → start → lmx status --wait converged
//
// Because [Guest.Apply] runs the lmxd of the generation it applies, the base image, a VM created before lmx and a
// current VM take the same steps. lmxd installs the environment files, makes room in the store and builds the
// generation for the next boot. Apply prints the build lines on Stdout and Stderr and passes lmx warnings to Warn. A
// build that fails on a full disk adds a [DiskError] with the usage lmx reported; a package build that runs out of
// room before lmx runs says that the disk is full.
//
// A failure before the restart stops the transient lmxd and starts the guest's own lmx.service, if there is one; the
// VM is not restarted. The apply belongs to lmxd: cancellation runs lmx apply cancel and returns the context's error
// once lmx confirms the stop; a stop that cannot be confirmed is an error. After the restart, a [FinalizeError]
// means that the generation runs, but lmxd could not remove the older generations yet and tries again later. Any
// other end of the wait carries the conditions lmxd reported last.
//
// # Disk and status
//
// [Guest.Reserve] runs before an update stops a running guest, and Apply runs it before the generation's sources
// are fetched. lmxd collects unreferenced store paths below [domain.DiskCollectPercent] free, and Reserve returns a
// [DiskError] when less than [domain.DiskMinimumPercent] stays free. Every other outcome, including a guest without
// lmx, is not an error: the apply makes room anyway.
//
// [Guest.Status] reads the address on Lima's shared network and the store disk usage with one bounded lmx status
// call. A guest without lmx gets a notice to run limanix update; any other failure leaves the status empty. The
// listing layer propagates cancellation of the parent operation. The client must support concurrent status calls.
//
// # Checks
//
// [Guest.Doctor] and [Guest.NetCheck] run lmx doctor and lmx net check within 30 seconds and return their [Check]
// records unchanged. lmx exits with status 1 when a check failed, but its answer is still a success.
//
// # Sessions
//
// [Guest.Shell] changes to the development user's home and preserves literal command arguments and exit status.
// An empty argument list opens a login shell. The management account and the development account are distinct.
//
// Read apply.go for the steps, follow.go for the build output and cancellation, lmx.go for the contract, status.go,
// reserve.go, and check.go for the queries, and shell.go for user switching.
package guest
