// Package vm coordinates VM lifecycle operations across host services.
//
// [Manager] uses the services in [Dependencies] to manage state, homes, Lima, and guest provisioning.
// [New] rejects missing or typed-nil services.
//
// # Create and update
//
// Create and Update hold the VM lock while preparing and applying a generation.
// A successful build selects the generation through an atomic relative current symlink, then restarts the VM.
// A running VM builds before restarting unless its disk or memory must grow.
// A stopped VM or one that needs growth receives its new Lima settings before the build.
// Update preserves identity and home and rejects disk shrinkage or an unknown current disk size.
//
// A failed build keeps the current generation and installed environment.
// Lima settings applied before the build remain in place.
// A confirmed successful build is selected even after cancellation, but cancellation prevents the next stop.
// A generation that runs but cannot yet be finalized is saved as ready with a warning.
//
// # Recovery and deletion
//
// Create rolls back local resources before backend creation when possible.
// Once backend work starts, failures retain ownership and inputs for recovery.
// Update discards uncommitted inputs and prunes old inputs only after saving a ready record.
// Pruning failures warn without failing the update.
// Guest generations are removed by the guest's own lifecycle.
//
// Delete holds the VM lock and reads identity independently of runtime state.
// It removes the backend, preserves or removes the managed home, then removes VM records.
// Force does not imply home removal.
// Persisted failures contain recovery guidance; detailed errors return to the caller.
//
// # Queries
//
// [Manager.Fetch] probes only the requested guest.
// [Manager.FetchAll] preserves row order and damaged records with bounded parallel guest probes.
// [Manager.Doctor] and [Manager.NetworkCheck] check a running, idle guest with lmx installed.
// TCP network checks also attempt a connection from the host within three seconds.
// These checks and [Manager.Shell] do not acquire the operation lock.
// Lima power state and Limanix operation state remain separate in [Info].
package vm
