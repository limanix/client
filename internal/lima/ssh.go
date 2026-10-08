package lima

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os/exec"
	"strings"
)

// Run executes a management command without a host shell or a build timeout.
//
// A captured run also returns standard output when the command fails, for callers that read an answer the
// failing command printed.
func (client *Client) Run(ctx context.Context, name string, args []string, capture bool) (string, error) {
	if !capture {
		return "", client.Stream(ctx, name, args, client.Stdout)
	}

	var stdout bytes.Buffer
	err := client.run(ctx, name, args, &stdout, io.Discard)

	return stdout.String(), err
}

// Stream executes a management command and writes its standard output to stdout as it arrives; standard error
// reaches the client's diagnostics, as for an uncaptured Run.
func (client *Client) Stream(ctx context.Context, name string, args []string, stdout io.Writer) error {
	return client.run(ctx, name, args, stdout, client.Stderr)
}

func (client *Client) run(ctx context.Context, name string, args []string, stdout, diagnostics io.Writer) (failure error) {
	defer func() {
		failure = wrapOperation("SSH", failure)
	}()

	if len(args) == 0 {
		return ErrEmptyCommand
	}

	command, err := client.session(ctx, name, args, false)
	if err != nil {
		return err
	}

	configureProcessGroup(command)

	stderr := &tailBuffer{limit: 64 * 1024}
	command.Stdout = stdout
	command.Stderr = io.MultiWriter(diagnostics, stderr)

	if err = command.Run(); err != nil {
		return managementFailure(ctx, command, stderr, err)
	}

	return nil
}

func managementFailure(ctx context.Context, command *exec.Cmd, stderr *tailBuffer, err error) error {
	if ctx.Err() != nil {
		// WaitDelay kills SSH itself; helpers can remain in its process group.
		return errors.Join(ctx.Err(), killProcessGroup(command))
	}

	if exit, ok := errors.AsType[*exec.ExitError](err); ok {
		return &CommandError{
			Exit:   exit,
			Detail: strings.TrimSpace(string(stderr.data)),
		}
	}

	return err
}

// Shell inherits the controlling terminal and preserves the guest's exit status.
func (client *Client) Shell(ctx context.Context, name string, args []string) (status int, failure error) {
	defer func() {
		failure = wrapOperation("SSH", failure)
	}()

	command, err := client.session(ctx, name, args, true)
	if err != nil {
		return 0, err
	}

	command.Stdin = client.Stdin
	command.Stdout = client.Stdout
	command.Stderr = client.Stderr

	err = command.Run()

	switch {
	case err == nil:
		return 0, nil
	case ctx.Err() != nil:
		return 0, ctx.Err()
	}

	if exit, ok := errors.AsType[*exec.ExitError](err); ok {
		return exitStatus(exit), nil
	}

	return 0, err
}
