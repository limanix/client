package guest

import (
	"context"

	"github.com/limanix/client/internal/domain"
)

// Shell enters the regular user's home and preserves the command's exit status.
func (guest *Guest) Shell(ctx context.Context, name string, user domain.Username, command []string) (int, error) {
	return guest.client.Shell(ctx, name, userCommand(user, command))
}

func userCommand(user domain.Username, command []string) []string {
	if len(command) == 0 {
		// sudo resolves the account's configured login shell and enters its home.
		return []string{"sudo", "--login", "--user", string(user)}
	}

	var (
		bash = "/run/current-system/sw/bin/bash"
		args = []string{
			"sudo",
			"--set-home",
			"--user", string(user),
			"--", bash, "--login",
		}
	)

	args = append(args, "-c", `cd -- "$HOME" && exec "$@"`, "limanix-command")
	return append(args, command...)
}
