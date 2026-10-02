package nixos

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/limanix/client/internal/config"
	"github.com/limanix/client/internal/domain"
	"github.com/limanix/client/internal/modules"
)

func workspaceExecutable(t *testing.T, name string) string {
	t.Helper()
	path, err := exec.LookPath(name)
	if err != nil {
		t.Fatal(err)
	}
	return path
}

func workspaceProcess(t *testing.T, helper, summary string, env []string, args ...string) *exec.Cmd {
	t.Helper()
	script, err := resources.ReadFile("resources/base/" + helper + ".sh")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	command := exec.CommandContext(ctx, workspaceExecutable(t, "sh"), append([]string{"-c", string(script), "limanix-" + helper}, args...)...)
	command.Env = append([]string{"PATH=/usr/bin:/bin", "limanix_cat=" + workspaceExecutable(t, "cat"), "limanix_summary=" + summary}, env...)
	return command
}

func workspaceFixture(t *testing.T) string {
	t.Helper()
	summary := filepath.Join(t.TempDir(), "summary $literal 'quote'")
	if err := os.WriteFile(summary, []byte("VM: sample (arm64)\nUser: dev\nHome: /home/dev\nModules: none\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return summary
}

func TestWorkspaceHelpWorksWithoutOptionalModules(t *testing.T) {
	summary := workspaceFixture(t)
	for _, args := range [][]string{nil, {"--help"}, {"-h"}} {
		command := workspaceProcess(t, "help", summary, nil, args...)
		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("help failed: %v: %s", err, output)
		}
		for _, expected := range []string{"Modules: none", "limanix-info", "limanix-session NAME", "limanix shell NAME", "limanix update --config FILE"} {
			if !strings.Contains(string(output), expected) {
				t.Errorf("missing guidance %q: %s", expected, output)
			}
		}
	}
	command := workspaceProcess(t, "help", summary, nil, "--unknown")
	output, err := command.CombinedOutput()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 2 || !strings.Contains(string(output), "Usage:") {
		t.Fatalf("invalid helper arguments: %v, %s", err, output)
	}
	command = workspaceProcess(t, "help", "/missing/limanix-summary", nil)
	output, err = command.CombinedOutput()
	if !errors.As(err, &exit) || exit.ExitCode() != 1 || !strings.Contains(string(output), "metadata cannot be read") {
		t.Fatalf("missing metadata hidden: %v, %s", err, output)
	}
}

func TestWorkspaceInfoReportsMountAndServiceFailures(t *testing.T) {
	summary := workspaceFixture(t)
	for _, test := range []struct {
		name             string
		mounts, services int
		want             int
	}{
		{name: "healthy"},
		{name: "no-shares", mounts: 1},
		{name: "mount-failure", mounts: 2, want: 2},
		{name: "service-failure", services: 1, want: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			directory := t.TempDir()
			makeTool := func(name, text string) string {
				path := filepath.Join(directory, name)
				if err := os.WriteFile(path, []byte("#!"+workspaceExecutable(t, "sh")+"\n"+text), 0o700); err != nil {
					t.Fatal(err)
				}
				return path
			}
			mounts := makeTool("findmnt", "printf '%s\\n' '/workspace virtiofs rw'\nexit "+string(rune('0'+test.mounts))+"\n")
			services := makeTool("systemctl", "exit "+string(rune('0'+test.services))+"\n")
			kernel := makeTool("uname", "printf '%s\\n' 'Linux fixture'\n")
			command := workspaceProcess(t, "info", summary, []string{"limanix_findmnt=" + mounts, "limanix_systemctl=" + services, "limanix_uname=" + kernel})
			var diagnostics bytes.Buffer
			command.Stderr = &diagnostics
			output, err := command.Output()
			if test.want == 0 {
				if err != nil {
					t.Fatalf("info failed: %v: %s", err, diagnostics.String())
				}
				if !strings.Contains(string(output), "Kernel: Linux fixture") || !strings.Contains(string(output), "Failed system services") {
					t.Fatalf("missing overview: %s", output)
				}
				if test.mounts == 1 && !strings.Contains(string(output), "(none)") {
					t.Fatalf("missing shares became failure: %s", output)
				}
			} else {
				var exit *exec.ExitError
				if !errors.As(err, &exit) || exit.ExitCode() != test.want {
					t.Fatalf("status lost: expected %d, got %v", test.want, err)
				}
			}
		})
	}
}

func TestWorkspaceMetadataPreservesSelectedModulesWithoutEnvironment(t *testing.T) {
	cfg := config.Default()
	cfg.Env["PRIVATE_TOKEN"] = "never-in-workspace-metadata"
	cfg.NixOS.Modules = []domain.ModuleID{"lmx:git", "lmx:git"}
	flake, err := Prepare(cfg, filepath.Join(t.TempDir(), "runtime"), []modules.Source{{ID: "lmx:git"}, {ID: "lmx:git"}}, 501)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(flake, "runtime.json"))
	if err != nil {
		t.Fatal(err)
	}
	var runtime runtimeConfig
	if err := json.Unmarshal(data, &runtime); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(runtime.SelectedModules, cfg.NixOS.Modules) || strings.Contains(string(data), "never-in-workspace-metadata") {
		t.Fatalf("workspace metadata changed: %s", data)
	}
	for _, name := range []string{"workspace.nix", "help.sh", "info.sh"} {
		if _, err := os.Stat(filepath.Join(flake, name)); err != nil {
			t.Fatalf("base helper %s missing: %v", name, err)
		}
	}
	cfg.NixOS.Modules = nil
	flake, err = Prepare(cfg, filepath.Join(t.TempDir(), "runtime"), nil, 501)
	if err != nil {
		t.Fatal(err)
	}
	data, err = os.ReadFile(filepath.Join(flake, "runtime.json"))
	if err != nil || !bytes.Contains(data, []byte(`"selectedModules": []`)) {
		t.Fatalf("empty modules must remain a JSON list: %s, %v", data, err)
	}
}
