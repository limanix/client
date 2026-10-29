package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
)

var (
	errMissingConfig        = errors.New("required flag --config was not provided")
	errMissingModuleCommand = errors.New("a modules subcommand is required")
	errQuietJSON            = errors.New("--quiet cannot be combined with --json")
	errMissingVMName        = errors.New("a VM name is required")
	errDuplicateSession     = errors.New("--session may only be specified once")
	errMissingSession       = errors.New("--session requires a session name")
	errEmptySession         = errors.New("--session requires a nonempty session name")
	errSessionCommand       = errors.New("--session cannot be combined with a guest command")
)

// exitError carries either a usage failure or an unmodified guest exit status.
type exitError struct {
	code int
	err  error
}

func (e *exitError) Error() string {
	if e.err == nil {
		return ""
	}

	return e.err.Error()
}

func (e *exitError) Unwrap() error {
	return e.err
}

var exitStatuses = []ExitStatus{
	{Status: exitCode(0), What: "Success."},
	{Status: exitCode(1), What: "The command failed. `limanix doctor` and `limanix network check` also exit with `1`, " +
		"after printing every check, when one failed."},
	{Status: exitCode(2), What: "Usage error, such as an unknown command, a missing argument, a missing `--config`, " +
		"or `-q` with `--json`."},
	{Status: exitCode(130), What: "The command was interrupted: Ctrl-C, `SIGTERM`, or a closed terminal."},
	{What: "`limanix shell` exits with the status of the guest command or shell."},
}

func exitCode(code int) *int {
	return &code
}

func usageError(err error) error {
	return &exitError{
		code: 2,
		err:  err,
	}
}

func reportError(output io.Writer, err error) int {
	if exit, ok := errors.AsType[*exitError](err); ok {
		if exit.err != nil {
			_, _ = fmt.Fprintf(output, "limanix: %v\n", exit.err)
		}

		return exit.code
	}

	if errors.Is(err, context.Canceled) {
		_, _ = fmt.Fprintln(output, "limanix: interrupted.")
		return 130
	}

	_, _ = fmt.Fprintf(output, "limanix: %v\n", err)
	return 1
}
