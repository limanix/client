// Package cli implements Limanix's command-line interface and generated command reference.
//
// Handlers parse arguments, request services, and render results.
// VM operations belong to internal/vm; module imports belong to internal/modules.
//
// [Command] constructs the Cobra tree without querying Lima or opening state.
// [Dependencies] supplies lazy service factories, with defaults from internal/app.
// [Reference] renders the same commands without executing them.
//
// [Execute] returns an exit code rather than calling os.Exit.
// Success returns 0, usage errors return 2, ordinary failures return 1, and canceled foreground operations return 130.
// Shell commands retain the remote exit status.
// Doctor and network check print all checks, then return 1 if any check failed.
//
// Normal commands cancel on SIGINT and SIGTERM.
// Interactive SSH handles foreground SIGINT itself.
// The hidden hostagent command owns its shutdown and writes JSON diagnostics.
package cli
