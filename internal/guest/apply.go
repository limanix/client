package guest

import (
	"context"

	"github.com/limanix/client/internal/domain"
)

// Apply installs global ENV before rebuilding; a failed rebuild never reboots.
func (guest *Guest) Apply(ctx context.Context, name string, user domain.Username) error {
	// A low disk does not block the apply: a smaller selection may still fit, and a failed build reports the usage.
	_ = guest.Reserve(ctx, name)
	if err := ctx.Err(); err != nil {
		return err
	}

	if err := guest.installEnvironment(ctx, name); err != nil {
		return guest.explainFailure(ctx, name, err)
	}

	if err := guest.buildGeneration(ctx, name); err != nil {
		return guest.explainFailure(ctx, name, err)
	}

	if err := guest.client.Stop(ctx, name); err != nil {
		return err
	}

	if err := guest.client.Start(ctx, name); err != nil {
		return err
	}

	_, err := guest.client.Run(ctx, name, userCommand(user, []string{"true"}), true)
	return err
}

func (guest *Guest) installEnvironment(ctx context.Context, name string) error {
	directory := []string{
		"sudo", "install",
		"-d", "-m", "0755",
		"/etc/limanix",
	}

	if _, err := guest.client.Run(ctx, name, directory, true); err != nil {
		return err
	}

	for _, file := range []string{"environment", "environment.sh"} {
		command := []string{
			"sudo", "install",
			"-m", "0644",
			"/mnt/limanix/" + file,
			"/etc/limanix/" + file,
		}

		if _, err := guest.client.Run(ctx, name, command, true); err != nil {
			return err
		}
	}

	return nil
}
