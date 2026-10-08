package vm

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"os"

	"github.com/limanix/client/internal/config"
	"github.com/limanix/client/internal/domain"
	"github.com/limanix/client/internal/filesystem"
	"github.com/limanix/client/internal/guest"
	"github.com/limanix/client/internal/lima"
)

// Manager coordinates application operations using explicitly supplied services.
type Manager struct {
	backend Backend
	store   Store
	homes   Homes
	guest   Guest

	generations *generationBuilder
	hostUID     int

	// dial opens the connection of a TCP network check from the Mac.
	dial func(context.Context, string, string) (net.Conn, error)

	// Warn receives the problems of a create or update that do not fail it.
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
		generations: newGenerationBuilder(deps),
		dial:        (&net.Dialer{Timeout: connectTimeout}).DialContext,
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

	if err := m.generations.modules.Check(ctx, cfg.NixOS.Modules); err != nil {
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
	instances, err := m.backend.FetchAll(ctx)
	if err != nil {
		return lima.Instance{}, err
	}

	for _, instance := range instances {
		if instance.Name == identity.LimaName() {
			return instance, nil
		}
	}

	return lima.Instance{}, &InstanceError{
		Name:  identity.Name,
		Cause: ErrBackendMissing,
	}
}

func (m *Manager) applyGuest(ctx context.Context, instance domain.Instance) error {
	name := instance.Identity.LimaName()
	if err := m.backend.Start(ctx, name); err != nil {
		return err
	}

	return m.guest.Apply(ctx, name, instance.Generation, instance.Identity.Arch)
}

// recordReady saves the VM as ready after a successful apply, or as failed after a failed one. A generation that runs
// although lmxd could not finalize it yet is ready; the warning follows the record.
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
