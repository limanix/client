// Package hostagent runs Lima's persistent host agent inside the Limanix executable.
//
// [Command] is a hidden subprocess interface invoked by Lima.
// It validates instance paths, holds a PID-file lock, and serves the instance's Unix API socket.
// The instance directory requires 0700 permissions; the socket uses 0600.
//
// SIGINT, SIGTERM, and parent-context cancellation request orderly shutdown.
// Lima's context stays live until its cleanup finishes.
// Output writers serialize concurrent writes, and the CLI configures JSON diagnostics.
//
// Cleanup checks ownership before removing PID files and sockets.
// Replacement paths are preserved and reported as errors.
package hostagent
