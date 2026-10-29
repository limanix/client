// Package filesystem provides checked host paths and atomic file replacement.
// Callers decide ownership, permissions, and locking policy.
//
// [Resolve] resolves existing symlinks before later path components, including "..", and permits a missing tail.
// [CheckDirectory] rejects a final symlink or non-directory and accepts absence.
// [OpenRegular] refuses a final symlink or special file.
// These checks cannot prevent another process from replacing parent directories between operations.
//
// [WriteFileAtomic] preserves old contents until rename.
// A directory sync error can be returned after replacement has occurred.
// Cleanup errors are retained with the primary failure.
// A zero mode preserves existing permissions, with 0600 used for new files or an existing zero mode.
// Replacing a regular read-only file requires a writable parent, not write access to the old contents.
// Final symlinks and special-file destinations are rejected.
package filesystem
