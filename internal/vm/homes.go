package vm

import (
	"context"
	"errors"
	"io/fs"
)

// KeptHome is a managed home that outlived its VM.
type KeptHome struct {
	Name      string `json:"name" doc:"As NAME."`
	Home      string `json:"home" doc:"As PATH."`
	CreatedAt string `json:"created_at" doc:"When the VM was created, in RFC 3339."`
	Size      *int64 `json:"size" doc:"Bytes that its files take on the disk, or null when the folder is gone."`
}

// KeptHomes lists the homes that deletion preserved; a damaged record or home warns and is skipped.
func (m *Manager) KeptHomes(ctx context.Context) ([]KeptHome, error) {
	names, err := m.store.KeptHomes()
	if err != nil {
		return nil, err
	}

	homes := make([]KeptHome, 0, len(names))

	for _, name := range names {
		identity, err := m.store.KeptHome(name)
		if err != nil {
			m.Warn("%v", err)
			continue
		}

		home := KeptHome{Name: name, Home: identity.Home, CreatedAt: identity.CreatedAt}

		size, err := m.homes.Size(ctx, identity)

		switch {
		case err == nil:
			home.Size = &size
		case ctx.Err() != nil:
			return nil, ctx.Err()
		case !errors.Is(err, fs.ErrNotExist):
			m.Warn("%v", err)
			continue
		}

		homes = append(homes, home)
	}

	return homes, nil
}

// RemoveHome deletes a kept home with its files and its record.
func (m *Manager) RemoveHome(ctx context.Context, name string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	identity, err := m.store.KeptHome(name)
	if err != nil {
		return err
	}

	return m.releaseHome(identity, true)
}
