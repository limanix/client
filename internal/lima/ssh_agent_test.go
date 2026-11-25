package lima

import (
	"crypto/ed25519"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
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
	// -G prints the options OpenSSH will use, without making a connection.
	output, err := exec.CommandContext(t.Context(), command.Path, append([]string{"-G"}, command.Args[1:]...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("read OpenSSH options: %v\n%s", err, output)
	}
	config := "\n" + string(output)
	for _, line := range []string{
		"user another-dev", "forwardagent yes", "hostkeyalias " + host,
		"userknownhostsfile " + pin, "globalknownhostsfile none",
	} {
		if !strings.Contains(config, "\n"+line+"\n") {
			t.Errorf("SSH config lacks %q", line)
		}
	}
	if !strings.Contains(config, "\nstricthostkeychecking true\n") && !strings.Contains(config, "\nstricthostkeychecking yes\n") {
		t.Fatalf("SSH host key checking is not strict: %s", config)
	}
	if !strings.Contains(config, "\nupdatehostkeys false\n") && !strings.Contains(config, "\nupdatehostkeys no\n") {
		t.Fatalf("SSH can update the pinned host key: %s", config)
	}
	if strings.Contains(config, "\ncontrolpath ") {
		t.Fatalf("SSH reuses a control connection: %s", config)
	}
}
