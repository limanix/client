// Package vm coordinates VM lifecycle operations across host services.
//
// [Manager] orders effects; it does not implement the Lima backend, filesystem records, or guest provisioning commands.
// [Dependencies] makes those boundaries explicit. Use [New]; missing or typed-nil services are programming errors,
// while operations return input, host, and backend failures.
//
// # Create and update
//
//	Create                              Update
//	config + preflight                  config + preflight
//	        ↓                                   ↓
//	VM lock                             VM lock
//	        ↓                                   ↓
//	generation + home                   identity + disk checks
//	        ↓                                   ↓
//	save creating                       prepare generation → save updating
//	        ↓                                   ↓
//	Lima create                         stop for disk or memory growth
//	        ↓                                   ↓
//	start                               edit → start, if stopped
//	        ↓                                   ↓
//	build → select current              build → select current
//	        ↓                                   ↓
//	stop → start → converge             stop → edit → start → converge
//	        ↓                                   ↓
//	save ready                          save ready → prune old inputs
//
// Create and Update hold the VM lock while applying a generation. The read-only mount covers all input generations;
// after a successful build, an atomic relative current symlink selects the generation for status and observation.
// A running VM whose disk and memory do not grow builds first and restarts once; that stop applies the Lima settings.
// A stopped VM, or a running one whose disk or memory grows, gets the new Lima settings before the build boots it, so
// an update also repairs a VM that cannot start with its old settings, such as an SSH port another program took.
// A failed build keeps current and the installed environment; Lima settings applied before the build remain.
// A confirmed successful build is selected even if the caller cancels, but cancellation prevents the next stop.
// Failures while preparing boot entries, installing environment, editing Lima or restarting are not rolled back.
// Update preserves identity and home and refuses a disk shrink or an unknown current disk size.
//
// # Failure and deletion boundaries
//
// Creation rolls back locally prepared resources before backend creation when possible. Once backend operations have begun,
// failures retain ownership and inputs for recovery. Update discards uncommitted inputs but preserves a generation that may
// already be referenced by state. Old inputs are pruned only after a successful ready record; pruning failures are
// reported through [Manager.Warn] and leave the operation successful. The guest owner removes replaced guest
// generations itself before the guest apply returns. A generation it could not finalize yet is ready, with a warning.
//
// Delete acquires the VM lock and reads identity independently of runtime state. It removes the backend first, then either
// preserves home ownership or removes the exact managed home, and finally removes VM records. Force and home removal
// are separate choices; force does not imply deleting the home.
//
// # Queries and entry points
//
// [Manager.FetchAll] combines state records with one backend listing and bounded parallel guest status probes, preserving row
// order and damaged records. [Manager.Doctor] and [Manager.NetworkCheck] read one VM the same way and return a [Report]:
// the host's vm and address checks, the guest's check records unchanged, and for a TCP port a connection from the Mac
// within three seconds. A VM that is not running or that another command changes, and a guest without lmx, are not
// asked. [Manager.Shell], Doctor, and NetworkCheck do not acquire the exclusive operation lock.
// Lima power state and Limanix operation state remain separate in [Info].
//
// Read create.go, update.go, and delete.go for scenarios; generation.go for local inputs and cleanup;
// lifecycle.go for start, stop, and shell; list.go for the read model; and check.go for doctor and network check.
// Error causes are in error.go; runtime diagnostic details are returned to callers rather than copied into persisted recovery messages.
package vm
