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
//	Lima create                         stop if running → edit
//	        ↓                                   ↓
//	start → guest apply                 start → guest apply
//	        ↓                                   ↓
//	save ready                          save ready → prune old inputs
//
// Create and Update load configuration and run preflight before taking the VM lock. Update checks saved identity and
// backend disk size under that lock, makes room on a running guest whose disk does not grow, warns when the guest
// disk stays nearly full, then rechecks the live backend before editing. It preserves identity and home
// and refuses a disk shrink or an unknown current disk size.
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
