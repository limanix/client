package vm

import (
	"context"
	"fmt"
	"io"

	"github.com/limanix/client/internal/domain"
	"github.com/limanix/client/internal/forward"
	"github.com/limanix/client/internal/lima"
)

// Forward brings ports of a running VM to localhost on the Mac until ctx ends or the connection to the VM is lost.
func (m *Manager) Forward(ctx context.Context, name domain.VMName, ports []forward.Port, out io.Writer) error {
	identity, err := m.store.LoadIdentity(name)
	if err != nil {
		return err
	}

	actual, err := m.requireLima(ctx, identity)
	if err != nil {
		return err
	}

	if actual.Status != lima.Running {
		return &InstanceError{
			Name:  name,
			Cause: ErrNotRunning,
		}
	}

	connection, err := m.backend.Dial(ctx, identity.LimaName())
	if err != nil {
		return err
	}
	defer func() { _ = connection.Close() }()

	if _, err = fmt.Fprintf(out, "Forwarding ports of %s to localhost. Press Ctrl-C to stop.\n", name); err != nil {
		return err
	}

	return forward.Run(ctx, connection, ports, out)
}
