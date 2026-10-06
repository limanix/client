package guest

import (
	"context"
	"errors"
)

const systemProfile = "/nix/var/nix/profiles/system"

// Prune keeps only the booted system generation and removes store paths that nothing references.
func (guest *Guest) Prune(ctx context.Context, name string) error {
	err := guest.removeReplacedGenerations(ctx, name)
	if ctx.Err() != nil {
		return err
	}

	_, collect := guest.client.Run(ctx, name, collectGarbage, false)

	return errors.Join(err, collect)
}

// removeReplacedGenerations also drops older generations of the base image, which LimaNix never boots.
func (guest *Guest) removeReplacedGenerations(ctx context.Context, name string) error {
	remove := []string{"sudo", "nix-env", "--profile", systemProfile, "--delete-generations", "old"}
	if _, err := guest.client.Run(ctx, name, remove, true); err != nil {
		return err
	}

	// Boot entries must not offer generations whose store paths are about to be collected.
	refresh := []string{"sudo", systemProfile + "/bin/switch-to-configuration", "boot"}
	_, err := guest.client.Run(ctx, name, refresh, true)

	return err
}
