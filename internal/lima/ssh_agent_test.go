package lima

import (
	"crypto/ed25519"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/lima-vm/lima/v2/pkg/instance/hostname"
	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/limatype/dirnames"
	"github.com/lima-vm/lima/v2/pkg/limatype/filenames"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

func agentSSHInstance(t *testing.T) *limatype.Instance {
	t.Helper()
	const name = "limanix-test"
	instDir := filepath.Join(t.TempDir(), "Lima Data", name)
	if err := os.MkdirAll(instDir, 0o700); err != nil {
		t.Fatal(err)
	}
	return &limatype.Instance{
		Name: name, Dir: instDir, SSHAddress: "127.0.0.1", SSHLocalPort: 60022,
		Config: &limatype.LimaYAML{User: limatype.User{Name: new("limanix-admin")}},
	}
}

func TestAgentSSHRequiresPinnedHostKey(t *testing.T) {
	client := NewClient(nil)
	instance := agentSSHInstance(t)
	if _, err := client.agentSSHCommand(t.Context(), instance, "dev", nil); !errors.Is(err, errUnpinnedAgentVM) {
		t.Fatalf("missing host key: %v", err)
	}
	if err := os.WriteFile(filepath.Join(instance.Dir, filenames.SSHKnownHosts), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := client.agentSSHCommand(t.Context(), instance, "dev", nil); !errors.Is(err, errUnpinnedAgentVM) {
		t.Fatalf("empty host key file: %v", err)
	}
}

func TestAgentSSHUsesPinnedDirectConnection(t *testing.T) {
	if _, err := exec.LookPath("ssh"); err != nil {
		t.Skip("OpenSSH is unavailable")
	}
	t.Setenv("LIMA_HOME", t.TempDir())
	t.Setenv("SSH", "ssh -o StrictHostKeyChecking=no -o ControlMaster=auto")
	configDir, err := dirnames.LimaConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	if err = os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(configDir, filenames.UserPrivateKey), []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	instance := agentSSHInstance(t)
	key, _, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	publicKey, err := ssh.NewPublicKey(key)
	if err != nil {
		t.Fatal(err)
	}
	pin := filepath.Join(instance.Dir, filenames.SSHKnownHosts)
	host := hostname.FromInstName(instance.Name)
	if err = os.WriteFile(pin, []byte(knownhosts.Line([]string{host}, publicKey)+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	client := NewClient(nil)
	command, err := client.agentSSHCommand(t.Context(), instance, "another-dev", nil)
	if err != nil {
		t.Fatal(err)
	}
	args := command.Args[1:]
	if len(args) < 2 || !slices.Equal(args[:2], []string{"-F", os.DevNull}) {
		t.Fatalf("SSH command must ignore host configuration: %q", args)
	}
	options := make(map[string][]string)
	for i := 2; i+1 < len(args) && args[i] == "-o"; i += 2 {
		name, value, ok := strings.Cut(args[i+1], "=")
		if !ok {
			t.Fatalf("invalid SSH option: %q", args[i+1])
		}
		name = strings.ToLower(name)
		options[name] = append(options[name], value)
	}
	for name, want := range map[string][]string{
		"user":                  {"another-dev"},
		"forwardagent":          {"yes"},
		"stricthostkeychecking": {"yes"},
		"hostkeyalias":          {host},
		"userknownhostsfile":    {`"` + pin + `"`},
		"globalknownhostsfile":  {"none"},
		"updatehostkeys":        {"no"},
		"controlmaster":         nil,
		"controlpath":           nil,
		"controlpersist":        nil,
	} {
		if got := options[name]; !slices.Equal(got, want) {
			t.Errorf("SSH option %s = %q, want %q", name, got, want)
		}
	}

	t.Run("OpenSSH parses the known hosts path", func(t *testing.T) {
		// Some CI containers run with a UID that has no passwd entry. The argument checks above still run there.
		probe := exec.CommandContext(t.Context(), command.Path, append([]string{"-G"}, args...)...)
		probe.Env = append(os.Environ(), "LC_ALL=C")
		output, err := probe.CombinedOutput()
		if err != nil {
			if strings.Contains(string(output), "No user exists for uid ") {
				t.Skip(strings.TrimSpace(string(output)))
			}
			t.Fatalf("read OpenSSH options: %v\n%s", err, output)
		}
		if !strings.Contains("\n"+string(output), "\nuserknownhostsfile "+pin+"\n") {
			t.Fatalf("OpenSSH did not parse the pinned file path: %s", output)
		}
	})
}
