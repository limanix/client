package vm

import (
	"context"
	"errors"

	"github.com/limanix/client/internal/domain"
	"github.com/limanix/client/internal/lima"
)

// Start starts the backend independently of the mutable runtime record.
func (m *Manager) Start(ctx context.Context, name domain.VMName) error {
	if err := m.lifecycle(ctx, name, m.backend.Start); err != nil {
		return err
	}

	m.refreshSSHConfig(ctx)

	return nil
}

// Stop preserves the guest disk and host home.
func (m *Manager) Stop(ctx context.Context, name domain.VMName) error {
	return m.lifecycle(ctx, name, m.backend.Stop)
}

func (m *Manager) lifecycle(ctx context.Context, name domain.VMName, operation func(context.Context, string) error) (err error) {
	lock, err := m.store.InstanceLock(ctx, name)
	if err != nil {
		return err
	}

	defer func() {
		err = errors.Join(err, lock.Close())
	}()

	identity, err := m.store.LoadIdentity(name)
	if err != nil {
		return err
	}

	if _, err = m.requireLima(ctx, identity); err != nil {
		return err
	}

	return operation(ctx, identity.LimaName())
}

// Shell passes command arguments unchanged to the configured development user.
func (m *Manager) Shell(ctx context.Context, name domain.VMName, command []string) (int, error) {
	return m.shell(ctx, name, command, false)
}

// ShellWithAgent opens a development session with access to the Mac's SSH agent.
func (m *Manager) ShellWithAgent(ctx context.Context, name domain.VMName, command []string) (int, error) {
	return m.shell(ctx, name, command, true)
}

func (m *Manager) shell(ctx context.Context, name domain.VMName, command []string, forwardAgent bool) (int, error) {
	identity, err := m.store.LoadIdentity(name)
	if err != nil {
		return 1, err
	}

	actual, err := m.requireLima(ctx, identity)
	if err != nil {
		return 1, err
	}

	if actual.Status != lima.Running {
		return 1, &InstanceError{
			Name:  name,
			Cause: ErrNotRunning,
		}
	}

	if forwardAgent {
		return m.guest.ShellWithAgent(ctx, identity.LimaName(), identity.Username, command)
	}

	return m.guest.Shell(ctx, identity.LimaName(), identity.Username, command)
}
