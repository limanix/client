package vm

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net"
	"os"

	"github.com/limanix/client/internal/config"
	"github.com/limanix/client/internal/domain"
	"github.com/limanix/client/internal/filesystem"
	"github.com/limanix/client/internal/lima"
)

// Manager coordinates VM operations across host services.
type Manager struct {
	backend Backend
	store   Store
	homes   Homes
	guest   Guest

	generations *generationBuilder
	hostUID     int
	sshConfig   string

	// dial and listen allow host network checks without opening real ports in tests.
	dial   func(context.Context, string, string) (net.Conn, error)
	listen func(string, string) (net.Listener, error)

	// Warn receives failures that do not stop an operation.
	Warn func(string, ...any)
}

// New creates a lifecycle manager without inspecting the host or creating state.
func New(deps Dependencies) *Manager {
	switch {
	case missingDependency(deps.Store):
		panic("vm: missing state store")
	case missingDependency(deps.Backend):
		panic("vm: missing backend")
	case deps.Modules == nil:
		panic("vm: missing module registry")
	case missingDependency(deps.Homes):
		panic("vm: missing managed-home service")
	case missingDependency(deps.Guest):
		panic("vm: missing guest service")
	}

	return &Manager{
		store:       deps.Store,
		backend:     deps.Backend,
		homes:       deps.Homes,
		guest:       deps.Guest,
		hostUID:     deps.HostUID,
		sshConfig:   deps.SSHConfig,
		generations: newGenerationBuilder(deps),
		dial:        (&net.Dialer{Timeout: connectTimeout}).DialContext,
		listen:      net.Listen,
		Warn:        log.New(os.Stderr, "limanix: warning: ", 0).Printf,
	}
}

func (m *Manager) preflight(ctx context.Context, cfg config.Config) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if m.hostUID == 0 {
		return ErrRootUser
	}

	if err := m.generations.modules.Check(ctx, cfg.Selection().Modules); err != nil {
		return err
	}

	if err := m.backend.Preflight(ctx, cfg); err != nil {
		return err
	}

	for _, mount := range cfg.Mounts {
		if _, err := filesystem.RequireDirectory(mount.Source); err != nil {
			return err
		}
	}

	if cfg.Shell.Dotfiles != "" {
		if _, err := filesystem.RequireDirectory(string(cfg.Shell.Dotfiles)); err != nil {
			return fmt.Errorf("shell.dotfiles: %w", err)
		}
	}

	return nil
}

func (m *Manager) recordFailure(instance domain.Instance, cause error) error {
	path, err := m.store.RecordPath(instance.Identity.Name)
	if err != nil {
		return errors.Join(cause, err)
	}

	message := fmt.Sprintf("Operation failed. VM state file: '%s'. Fix the reported issue before retrying update or delete.", path)
	instance.MarkFailed(message)

	return errors.Join(cause, m.store.Save(instance))
}

func (m *Manager) requireLima(ctx context.Context, identity domain.Identity) (lima.Instance, error) {
	instance, err := m.backend.Fetch(ctx, identity.LimaName())
	if errors.Is(err, fs.ErrNotExist) {
		return lima.Instance{}, &InstanceError{
			Name:  identity.Name,
			Cause: ErrBackendMissing,
		}
	}

	return instance, err
}
