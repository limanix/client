// Package guest builds generations through lmx, checks convergence, and opens development-user sessions.
//
// [Guest] uses the management connection in [Client].
// It reads version 1 of the lmx host contract; a valid JSON response takes precedence over the command's exit status.
// Contract examples are kept in testdata.
//
// # Applying generations
//
// [Guest.Build] temporarily runs the lmxd packaged by the requested generation at /mnt/limanix/<generation>.
// It builds the next system and installs its environment after a successful build.
// Build output goes to Stderr; lmx warnings go to Warn.
// A failed build attempts to restore the guest's own lmx.service.
//
// Cancellation requests lmx apply cancel and waits for a confirmed outcome.
// A confirmed successful apply remains successful after cancellation.
// An unconfirmed stop is an error.
// The VM manager selects the built generation, restarts the VM, and calls [Guest.Converge].
// A [FinalizeError] means the generation runs but lmxd has not yet removed older generations.
// Other convergence failures include the conditions last reported by lmxd.
//
// # Status and checks
//
// [Guest.Reserve] asks lmxd to free store space and returns [DiskError] when the remaining space is low.
// Build continues after reserve failures; the generation's lmxd tries again during apply.
// A build failure may also include [DiskError] with the reported usage.
//
// [Guest.Status] reads shared-network IPv4 and store usage within ten seconds.
// Missing lmx produces an update notice; other failures leave the status empty.
// The caller handles parent cancellation, and Client must support concurrent status requests.
// [Guest.Doctor] and [Guest.NetCheck] return lmx check records unchanged within thirty seconds.
//
// [Guest.Shell] uses the development user's home and preserves command arguments and exit status.
// An empty command opens a login shell.
package guest
