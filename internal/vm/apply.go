package vm

import (
	"context"
	"errors"

	"github.com/limanix/client/internal/domain"
	"github.com/limanix/client/internal/guest"
)

func (m *Manager) applyGuest(ctx context.Context, instance domain.Instance, template string, running bool) error {
	name := instance.Identity.LimaName()
	if !running {
		if err := m.backend.Edit(ctx, name, template); err != nil {
			return err
		}
		if err := m.backend.Start(ctx, name); err != nil {
			return err
		}
	}
	if err := m.guest.Build(ctx, name, instance.Generation, instance.Identity.Arch); err != nil {
		return err
	}
	if err := m.generations.selectCurrent(instance); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := m.backend.Stop(ctx, name); err != nil {
		return err
	}
	if err := m.backend.Edit(ctx, name, template); err != nil {
		return err
	}
	if err := m.backend.Start(ctx, name); err != nil {
		return err
	}

	return m.guest.Converge(ctx, name, instance.Generation)
}

// A generation that runs is ready even when finalization needs another attempt.
func (m *Manager) recordReady(instance domain.Instance, applied error) (domain.Instance, error) {
	unfinished, ok := errors.AsType[*guest.FinalizeError](applied)
	if applied != nil && !ok {
		return instance, m.recordFailure(instance, applied)
	}

	instance.MarkReady()
	if err := m.store.Save(instance); err != nil {
		return instance, err
	}

	if ok {
		m.Warn("VM '%s' is ready, but lmx reported: %s", instance.Identity.Name, unfinished.Message)
	}

	return instance, nil
}
