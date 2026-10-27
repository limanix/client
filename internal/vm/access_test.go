package vm

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/limanix/client/internal/lima"
)

// busyPorts makes the listed ports of the Mac look taken by another program.
func busyPorts(ports ...string) func(string, string) (net.Listener, error) {
	return func(_, address string) (net.Listener, error) {
		for _, port := range ports {
			if address == "127.0.0.1:"+port {
				return nil, errors.New("address already in use")
			}
		}
		return net.Listen("tcp", "127.0.0.1:0")
	}
}

func TestSSHPortsSkipTakenPortsAndStayOnUpdate(t *testing.T) {
	f := fixture(t)
	f.manager.listen = busyPorts("42022")
	first, path := f.create(t, "first")
	second, secondPath := f.create(t, "second")
	port := func(name string) int {
		return f.backend.instances[name].SSHPort
	}
	if port(first.Identity.LimaName()) != 42023 || port(second.Identity.LimaName()) != 42024 {
		t.Fatalf("expected the first free ports: %d, %d", port(first.Identity.LimaName()), port(second.Identity.LimaName()))
	}

	if _, err := f.manager.Update(context.Background(), path); err != nil {
		t.Fatal(err)
	}
	if port(first.Identity.LimaName()) != 42023 {
		t.Fatalf("an update moved the SSH port to %d", port(first.Identity.LimaName()))
	}

	older := f.backend.instances[first.Identity.LimaName()]
	older.SSHPort = 0
	f.backend.instances[first.Identity.LimaName()] = older
	f.manager.listen = busyPorts("42022", "42023")
	if _, err := f.manager.Update(context.Background(), path); err != nil {
		t.Fatal(err)
	}
	if port(first.Identity.LimaName()) != 42025 {
		t.Fatalf("a VM without a port must get the first free one, got %d", port(first.Identity.LimaName()))
	}

	stopped := f.backend.instances[second.Identity.LimaName()]
	stopped.Status = lima.Stopped
	f.backend.instances[second.Identity.LimaName()] = stopped
	f.manager.listen = busyPorts("42024")
	if _, err := f.manager.Update(context.Background(), secondPath); err != nil {
		t.Fatal(err)
	}
	if port(second.Identity.LimaName()) != 42022 {
		t.Fatalf("a stopped VM whose port another program took must move to a free one, got %d", port(second.Identity.LimaName()))
	}
	running := f.backend.instances[first.Identity.LimaName()]
	running.Status = lima.Running
	f.backend.instances[first.Identity.LimaName()] = running
	f.manager.listen = busyPorts("42025")
	if _, err := f.manager.Update(context.Background(), path); err != nil {
		t.Fatal(err)
	}
	if port(first.Identity.LimaName()) != 42025 {
		t.Fatalf("a running VM holds its own port and must keep it, got %d", port(first.Identity.LimaName()))
	}
}

func TestSSHConfigFollowsTheVMs(t *testing.T) {
	f := fixture(t)
	f.manager.listen = busyPorts()
	f.manager.sshConfig = filepath.Join(f.root, "ssh", "config")
	read := func() string {
		t.Helper()
		data, err := os.ReadFile(f.manager.sshConfig)
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}

	alpha, _ := f.create(t, "alpha")
	f.create(t, "beta")
	text := read()
	user := "  User " + string(alpha.Identity.Username) + "\n"
	if !strings.Contains(text, "Host alpha.limanix\n  HostName 127.0.0.1\n  Port 42022\n"+user) ||
		!strings.Contains(text, "Host beta.limanix\n  HostName 127.0.0.1\n  Port 42023\n") ||
		!strings.Contains(text, `IdentityFile "/lima/_config/user"`) {
		t.Fatalf("unexpected SSH settings:\n%s", text)
	}

	if _, err := f.manager.Delete(context.Background(), "beta", false, true); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(read(), "beta.limanix") {
		t.Fatalf("a deleted VM stayed in the SSH settings:\n%s", read())
	}

	if err := os.Remove(f.manager.sshConfig); err != nil {
		t.Fatal(err)
	}
	if err := f.manager.Start(context.Background(), "alpha"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(read(), "Host alpha.limanix\n") {
		t.Fatalf("start must restore the SSH settings:\n%s", read())
	}
}

func TestNoFreeSSHPort(t *testing.T) {
	f := fixture(t)
	f.manager.listen = func(string, string) (net.Listener, error) {
		return nil, errors.New("address already in use")
	}
	_, err := f.manager.Create(context.Background(), f.writeConfig(t, f.configuration("crowded")))
	if !errors.Is(err, ErrNoSSHPort) {
		t.Fatalf("expected ErrNoSSHPort, got %v", err)
	}
	if len(f.backend.instances) != 0 {
		t.Fatal("a VM without an SSH port reached Lima")
	}
}
