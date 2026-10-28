// Package guest builds generations through lmx, waits for their convergence, and opens development-user sessions.
//
// [Guest] uses a [Client] management connection supplied by internal/lima. It does not allocate host homes,
// persist VM status, control the VM lifecycle, or construct flake inputs; those responsibilities belong to internal/vm, internal/state,
// and internal/nixos. It reads version 1 of the lmx host contract: a command answers with one JSON envelope, and
// the answer wins over the exit status. Copies of the host contract examples in testdata keep the
// decoders honest.
//
// # Building and checking a generation
//
//	lmx store reserve → stop the guest's lmx units
//	             ↓
//	nix build packages.<system>.lmx of the requested generation → /run/limanix-lmx
//	             ↓
//	run its lmxd as lmx-transient.service → lmx apply --follow
//
// [Guest.Build] runs the lmxd packaged by the requested generation at /mnt/limanix/<generation>.
// lmxd makes room in the store and builds that generation for the next boot, then installs its environment files.
// The build reads the candidate environment without replacing the running system's files before it succeeds.
// Build prints the build lines on Stderr and passes lmx warnings to Warn.
// A build that fails on a full disk adds a [DiskError] with the usage lmx reported; a package build that runs out of room before lmx runs says that the disk is full.
//
// The VM manager starts the guest when needed, calls Build, selects current after success, and owns the stop, Lima edit and start.
// It then calls [Guest.Converge], which waits for lmx status --wait converged.
// A [FinalizeError] means that the generation runs, but lmxd could not remove the older generations yet and tries again later.
// Any other end of the wait carries the conditions lmxd reported last.
//
// After a build failure, Build attempts to stop the transient lmxd and start the guest's own lmx.service, if there is one.
// The apply belongs to lmxd: cancellation runs lmx apply cancel; a confirmed successful apply still returns success, otherwise cancellation returns the context's error once lmx confirms the stop.
// A stop that cannot be confirmed is an error; the VM manager checks cancellation separately before restarting.
//
// # Disk and status
//
// Build calls [Guest.Reserve] before fetching the generation's sources.
// lmxd collects unreferenced store paths below [domain.DiskCollectPercent] free, and Reserve returns a [DiskError] when less than [domain.DiskMinimumPercent] stays free.
// Build continues after a low-disk result; the generation's lmxd makes room again during its apply.
// Every other outcome, including a guest without lmx, is not an error.
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
