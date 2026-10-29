// Package managedhome manages the host home allocation recorded in a VM's identity.
// The directory is home.root/<name>-<id>, separate from VM state and explicit mounts.
//
// Existing directories are never adopted, even when empty.
// Removal accepts absence but refuses redirected paths and non-directories.
// Guest-writable marker files are never used as ownership evidence.
package managedhome
