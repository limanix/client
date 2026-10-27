package sshconfig

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var hosts = []Host{
	{Name: "personal", User: "dev", Port: 42022, IdentityFile: "/Users/me/Library/Application Support/lima/_config/user"},
	{Name: "work", User: "anna", Port: 42023, IdentityFile: "/Users/me/.lima/_config/user"},
}

func TestRenderWritesOneBlockPerVM(t *testing.T) {
	text := string(Render(hosts))
	block := `
Host personal.limanix
  HostName 127.0.0.1
  Port 42022
  User dev
  IdentityFile "/Users/me/Library/Application Support/lima/_config/user"
  IdentitiesOnly yes
  PreferredAuthentications publickey
  StrictHostKeyChecking no
  UserKnownHostsFile /dev/null
  LogLevel ERROR
`
	if !strings.HasPrefix(text, header) || !strings.Contains(text, block) || strings.Count(text, "\nHost ") != 2 {
		t.Fatalf("unexpected file:\n%s", text)
	}
	if string(Render(nil)) != header {
		t.Fatalf("a file without VMs must keep only its header: %q", Render(nil))
	}
}

func TestOpenSSHReadsTheFile(t *testing.T) {
	ssh, err := exec.LookPath("ssh")
	if err != nil {
		t.Skip("ssh is not installed")
	}
	if output, err := exec.Command(ssh, "-G", "-F", os.DevNull, "localhost").CombinedOutput(); err != nil {
		t.Skipf("ssh cannot run here: %s", strings.TrimSpace(string(output)))
	}
	path := filepath.Join(t.TempDir(), "config")
	if err := Write(path, hosts); err != nil {
		t.Fatal(err)
	}
	output, err := exec.Command(ssh, "-G", "-F", path, "personal.limanix").Output()
	if exit, ok := errors.AsType[*exec.ExitError](err); ok {
		t.Fatalf("ssh -G rejected the file: %v: %s", err, exit.Stderr)
	}
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range []string{
		"hostname 127.0.0.1",
		"port 42022",
		"user dev",
		"identityfile /Users/me/Library/Application Support/lima/_config/user",
		"stricthostkeychecking false",
		"userknownhostsfile /dev/null",
	} {
		if !strings.Contains(string(output), "\n"+line+"\n") {
			t.Fatalf("ssh -G lacks %q:\n%s", line, output)
		}
	}
}

func TestWriteReplacesAPrivateFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ssh", "config")
	if err := Write(path, hosts); err != nil {
		t.Fatal(err)
	}
	if err := Write(path, hosts[1:]); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "personal.limanix") || !strings.Contains(string(data), "work.limanix") {
		t.Fatalf("the file kept a removed VM:\n%s", data)
	}
	for file, want := range map[string]os.FileMode{path: 0o600, filepath.Dir(path): 0o700} {
		info, err := os.Stat(file)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != want {
			t.Fatalf("%s has mode %v, want %v", file, info.Mode().Perm(), want)
		}
	}
}
