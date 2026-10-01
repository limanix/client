package nixos

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func sessionProcess(t *testing.T, command, providers string, names ...string) *exec.Cmd {
	t.Helper()
	script, err := resources.ReadFile("resources/base/session.sh")
	if err != nil {
		t.Fatal(err)
	}
	args := append([]string{"-c", string(script), "limanix-session"}, names...)
	process := exec.Command("/bin/sh", args...)
	process.Env = append(os.Environ(), "limanix_session_command="+command, "limanix_session_providers="+providers)
	return process
}

func TestSessionLauncherPreservesLiteralNameStreamsAndStatus(t *testing.T) {
	directory := t.TempDir()
	provider := filepath.Join(directory, "provider $literal 'quote'; script")
	err := os.WriteFile(provider, []byte("#!/bin/sh\nprintf '%s\\000' \"$@\"\nprintf '%s' 'provider diagnostic' >&2\nexit 17\n"), 0o700)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{
		"work",
		"--help",
		`spaces; $(touch unwanted) "quotes"`,
		"work;",
		`#(touch unwanted)#{session_name}#[red]`,
		"line one\nline two",
		"unicode ✓",
	} {
		process := sessionProcess(t, provider, "", name)
		process.Dir = directory
		var diagnostic bytes.Buffer
		process.Stderr = &diagnostic
		output, err := process.Output()
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != 17 {
			t.Fatalf("name %q: provider status changed: %v", name, err)
		}
		if string(output) != name+"\x00" || diagnostic.String() != "provider diagnostic" {
			t.Fatalf("name %q: provider arguments or streams changed: %q, %q", name, output, diagnostic.String())
		}
	}
	if _, err := os.Stat(filepath.Join(directory, "unwanted")); !os.IsNotExist(err) {
		t.Fatalf("session name was interpreted as shell code: %v", err)
	}
}

func TestSessionLauncherMissingProviderUsesCatalogSuggestions(t *testing.T) {
	for _, providers := range []string{"", "lmx:example, third-party:sessions"} {
		process := sessionProcess(t, "", providers, "work")
		var diagnostic bytes.Buffer
		process.Stderr = &diagnostic
		output, err := process.Output()
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != 127 || len(output) != 0 {
			t.Fatalf("missing provider: output %q, error %v", output, err)
		}
		message := diagnostic.String()
		if !strings.Contains(message, "No session provider is configured") {
			t.Fatalf("missing provider diagnostic: %q", message)
		}
		if providers == "" {
			if strings.Contains(message, "nixos.modules:") {
				t.Fatalf("invented provider suggestions: %q", message)
			}
		} else if !strings.Contains(message, providers) || !strings.Contains(message, "limanix update") {
			t.Fatalf("missing catalog suggestions or next step: %q", message)
		}
	}
}

func TestSessionLauncherRejectsInvalidNamesBeforeProvider(t *testing.T) {
	for _, names := range [][]string{nil, {""}, {"work", "another"}} {
		process := sessionProcess(t, "/does/not/exist", "lmx:example", names...)
		var diagnostic bytes.Buffer
		process.Stderr = &diagnostic
		output, err := process.Output()
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != 2 || len(output) != 0 || !strings.Contains(diagnostic.String(), "Usage: limanix-session") {
			t.Fatalf("invalid names %q: output %q, diagnostic %q, error %v", names, output, diagnostic.String(), err)
		}
	}
}
