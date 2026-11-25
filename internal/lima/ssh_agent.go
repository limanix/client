package lima

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"slices"

	"al.essio.dev/pkg/shellescape"
	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/sshutil"
)

var errUnpinnedAgentVM = errors.New("the VM has no pinned SSH host key. Stop and start it with this client before forwarding your agent")

func (client *Client) agentSSHCommand(ctx context.Context, inst *limatype.Instance, user string, args []string) (*exec.Cmd, error) {
	if err := validateSSHMetadata(inst); err != nil {
		return nil, err
	}

	pinned, err := sshutil.ReadPinnedHostKey(inst.Dir)
	if err != nil {
		return nil, err
	}
	if pinned == nil {
		return nil, errUnpinnedAgentVM
	}

	// Resolve OpenSSH directly: the SSH environment override can add options ahead of our checks.
	sshPath, err := exec.LookPath("ssh")
	if err != nil {
		return nil, fmt.Errorf("locate system SSH: %w", err)
	}
	sshExe := sshutil.SSHExe{Exe: sshPath}
	opts, err := sshutil.SSHOptsWithoutMultiplexing(ctx, sshExe, sshExe.Exe, inst.Dir, user, false)
	if err != nil {
		return nil, err
	}
	// If the pin file disappeared, Lima's option builder could fall back to an unchecked connection.
	if !slices.Contains(opts, "StrictHostKeyChecking=yes") {
		return nil, errUnpinnedAgentVM
	}

	opts = append(opts, "GlobalKnownHostsFile=none", "UpdateHostKeys=no", "ForwardAgent=yes")
	return client.sshExecCommand(ctx, sshExe, inst, opts, agentRemoteScript(args, os.Getenv("COLORTERM")), true), nil
}

func checkAgentSocket() error {
	path := os.Getenv("SSH_AUTH_SOCK")
	if path == "" {
		return errors.New("SSH_AUTH_SOCK is unset. Start an SSH agent on the host before forwarding it")
	}
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("access the host SSH agent socket from SSH_AUTH_SOCK: %w", err)
	}
	if info.Mode()&os.ModeSocket == 0 {
		return errors.New("SSH_AUTH_SOCK does not point to a Unix socket")
	}
	return nil
}

func agentRemoteScript(args []string, colorterm string) string {
	prefix := "exec "
	if colorterm == "truecolor" || colorterm == "24bit" {
		prefix += "env COLORTERM=" + colorterm + " "
	}
	if len(args) == 0 {
		return `cd -- "$HOME" && ` + prefix + `"$SHELL" -l`
	}

	command := []string{"/run/current-system/sw/bin/bash", "--login", "-c", `cd -- "$HOME" && exec -- "$@"`, "limanix-command"}
	return prefix + shellescape.QuoteCommand(append(command, args...))
}
