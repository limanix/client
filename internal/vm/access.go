package vm

import (
	"context"
	"errors"
	"net"
	"slices"
	"sort"
	"strconv"

	"github.com/limanix/client/internal/lima"
	"github.com/limanix/client/internal/sshconfig"
	"github.com/limanix/client/internal/state"
)

// The SSH ports of VMs stay below the ephemeral range of macOS, which starts at 49152.
const (
	sshPortFirst = 42022
	sshPortLast  = 49151
)

// A stopped VM may need a new port if another process took its previous one; a new port stays reserved until release.
func (m *Manager) sshPort(ctx context.Context, actual lima.Instance) (int, func() error, error) {
	if actual.SSHPort > 0 && (actual.Status == lima.Running || m.free(actual.SSHPort)) {
		return actual.SSHPort, func() error { return nil }, nil
	}

	instances, err := m.backend.FetchAll(ctx)
	if err != nil {
		return 0, nil, err
	}

	taken := make(map[int]bool, len(instances))
	for _, instance := range instances {
		taken[instance.SSHPort] = true
	}

	for port := sshPortFirst; port <= sshPortLast; port++ {
		if taken[port] || !m.free(port) {
			continue
		}

		reservation, err := m.store.PortLock(ctx, port)
		if errors.Is(err, state.ErrLockBusy) {
			continue
		}
		if err != nil {
			return 0, nil, err
		}

		instances, err = m.backend.FetchAll(ctx)
		if err != nil {
			return 0, nil, errors.Join(err, reservation.Close())
		}

		if !slices.ContainsFunc(instances, func(instance lima.Instance) bool { return instance.SSHPort == port }) {
			return port, reservation.Close, nil
		}

		if err = reservation.Close(); err != nil {
			return 0, nil, err
		}
	}

	return 0, nil, ErrNoSSHPort
}

func (m *Manager) free(port int) bool {
	listener, err := m.listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		return false
	}

	return listener.Close() == nil
}

// SSH configuration failures warn without failing the VM operation.
func (m *Manager) refreshSSHConfig(ctx context.Context) {
	if m.sshConfig == "" {
		return
	}

	if err := m.writeSSHConfig(ctx); err != nil {
		m.Warn("the SSH settings for editors were not updated: %v", err)
	}
}

func (m *Manager) writeSSHConfig(ctx context.Context) error {
	entries, err := m.store.FetchAll()
	if err != nil {
		return err
	}

	instances, err := m.backend.FetchAll(ctx)
	if err != nil {
		return err
	}

	ports := make(map[string]int, len(instances))
	for _, instance := range instances {
		ports[instance.Name] = instance.SSHPort
	}

	key, err := m.backend.UserKey(ctx)
	if err != nil {
		return err
	}

	hosts := []sshconfig.Host{}

	for _, entry := range entries {
		if entry.Identity == nil || ports[entry.Identity.LimaName()] == 0 {
			continue
		}

		hosts = append(hosts, sshconfig.Host{
			Name:         string(entry.Identity.Name),
			User:         string(entry.Identity.Username),
			Port:         ports[entry.Identity.LimaName()],
			IdentityFile: key.PrivateFile,
		})
	}

	sort.Slice(hosts, func(i, j int) bool {
		return hosts[i].Name < hosts[j].Name
	})

	return sshconfig.Write(m.sshConfig, hosts)
}
