package guest

import (
	"context"
	"os"

	"github.com/limanix/client/internal/domain"
)

// Shell enters the regular user's home and preserves the command's exit status.
func (guest *Guest) Shell(ctx context.Context, name string, user domain.Username, command []string) (int, error) {
	return guest.client.Shell(ctx, name, userCommand(user, os.Getenv("COLORTERM"), command))
}

func userCommand(user domain.Username, colorterm string, command []string) []string {
	sudo := []string{"sudo"}
	// SSH does not forward COLORTERM and sudo resets it, so a 24-bit terminal on the Mac passes it on.
	if colorterm == "truecolor" || colorterm == "24bit" {
		sudo = []string{"env", "COLORTERM=" + colorterm, "sudo", "--preserve-env=COLORTERM"}
	}

	if len(command) == 0 {
		// sudo resolves the account's configured login shell and enters its home.
		return append(sudo, "--login", "--user", string(user))
	}

	var (
		bash = "/run/current-system/sw/bin/bash"
		args = append(sudo,
			"--set-home",
			"--user", string(user),
			"--", bash, "--login",
		)
	)

	args = append(args, "-c", `cd -- "$HOME" && exec "$@"`, "limanix-command")
	return append(args, command...)
}
