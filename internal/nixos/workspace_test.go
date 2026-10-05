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
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

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
	command := exec.CommandContext(ctx, workspaceExecutable(t, "bash"), append([]string{"-c", string(script), "lmx " + helper}, args...)...)
	command.Env = []string{
		"PATH=/usr/bin:/bin",
		"limanix_cat=" + workspaceExecutable(t, "cat"),
		"limanix_summary=" + summary,
		"limanix_name=sample",
		"limanix_system=NixOS fixture, arm64",
		"limanix_modules=none",
	}
	if helper == "lmx" {
		for _, name := range []string{"help", "info", "welcome"} {
			data, err := resources.ReadFile("resources/base/" + name + ".sh")
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), name+".sh")
			if err := os.WriteFile(path, data, 0o600); err != nil {
				t.Fatal(err)
			}
			command.Env = append(command.Env, "limanix_"+name+"="+path)
		}
	}
	command.Env = append(command.Env, env...)
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
		for _, expected := range []string{"Modules: none", "lmx info", "lmx welcome", "limanix-session NAME", "limanix shell NAME", "limanix update --config FILE"} {
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

func workspaceTool(t *testing.T, name, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte("#!"+workspaceExecutable(t, "sh")+"\n"+body), 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestWorkspaceCommandDispatch(t *testing.T) {
	env := []string{
		"limanix_findmnt=" + workspaceTool(t, "findmnt", "exit 1\n"),
		"limanix_uname=" + workspaceTool(t, "uname", "printf '%s\\n' 'Linux fixture'\n"),
		"limanix_systemctl=" + workspaceTool(t, "systemctl", "exit 0\n"),
	}
	for _, test := range []struct {
		args []string
		want string
		code int
	}{
		{want: "LimaNix workspace"},
		{args: []string{"help"}, want: "LimaNix workspace"},
		{args: []string{"--help"}, want: "LimaNix workspace"},
		{args: []string{"-h"}, want: "LimaNix workspace"},
		{args: []string{"info"}, want: "Kernel: Linux fixture"},
		{args: []string{"welcome"}, want: "(none)"},
		{args: []string{"missing"}, want: "Usage: lmx", code: 2},
		{args: []string{"info", "extra"}, want: "Usage: lmx info", code: 2},
		{args: []string{"welcome", "extra"}, want: "Usage: lmx welcome", code: 2},
	} {
		t.Run(strings.Join(test.args, "/"), func(t *testing.T) {
			output, err := workspaceProcess(t, "lmx", workspaceFixture(t), env, test.args...).CombinedOutput()
			if test.code == 0 && err != nil {
				t.Fatalf("dispatch failed: %v: %s", err, output)
			}
			if test.code != 0 {
				var exit *exec.ExitError
				if !errors.As(err, &exit) || exit.ExitCode() != test.code {
					t.Fatalf("expected status %d, got %v: %s", test.code, err, output)
				}
			}
			if !strings.Contains(string(output), test.want) {
				t.Fatalf("missing %q: %s", test.want, output)
			}
		})
	}
}

func TestWorkspaceWelcomeWrapsValuesAndReportsMountFailures(t *testing.T) {
	longPath := "/workspace/" + strings.Repeat("project-", 16)
	unicodePath := "/workspace/" + strings.Repeat("界", 50)
	rows := longPath + "\trw\n" + unicodePath + "\tro\n/workspace/a $literal 'quote'\trw\n"
	meminfo := filepath.Join(t.TempDir(), "meminfo")
	if err := os.WriteFile(meminfo, []byte("MemTotal:        7969124 kB\nMemFree:          524288 kB\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	probes := map[string]string{
		"nproc": "printf '%s\\n' 4\n",
		"df":    "printf '%s\\n' 'Filesystem 1024-blocks Used Available Capacity Mounted on' '/dev/vda2 103081248 61234567 39800000 61% /'\n",
		"systemctl": "printf '%s\\n' '" + strings.Repeat("unit-", 12) + "a.service loaded failed failed A' " +
			"'" + strings.Repeat("unit-", 12) + "b.service loaded failed failed B'\n",
	}
	resources := "Resources  4 CPUs, 7.6 GiB memory, 38 GiB disk free"
	for _, test := range []struct {
		name           string
		mounts, parser int
		failedProbes   bool
		want, absent   []string
		code           int
	}{
		{name: "mounted", want: []string{"'quote'", resources, "▲ Failed:"}},
		{name: "no-shares", mounts: 1, want: []string{"(none)"}},
		{name: "mount-failure", mounts: 2, want: []string{"unavailable"}, code: 2},
		{name: "malformed-mount-data", parser: 4, want: []string{"unavailable"}, code: 4},
		{name: "failed-probes", failedProbes: true, want: []string{"Modules"}, absent: []string{"Resources", "failed service"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			env := []string{
				"TERM=xterm-256color",
				"NO_COLOR=1",
				"limanix_name=" + strings.Repeat("long-vm-name-", 10),
				"limanix_modules=" + strings.Repeat("lmx:console, ", 12) + "lmx:git",
				"limanix_findmnt=" + workspaceTool(t, "findmnt", "printf '%s\\n' '{\"filesystems\": []}'\nexit "+strconv.Itoa(test.mounts)+"\n"),
				"limanix_jq=" + workspaceTool(t, "jq", "cat <<'ROWS'\n"+rows+"ROWS\nexit "+strconv.Itoa(test.parser)+"\n"),
				"limanix_meminfo=" + meminfo,
			}
			if test.failedProbes {
				env[len(env)-1] = "limanix_meminfo=" + filepath.Join(t.TempDir(), "missing")
			}
			for name, body := range probes {
				if test.failedProbes {
					body = "exit 1\n"
				}
				env = append(env, "limanix_"+name+"="+workspaceTool(t, name, body))
			}
			output, err := workspaceProcess(t, "lmx", workspaceFixture(t), env, "welcome").CombinedOutput()
			if test.code == 0 && err != nil {
				t.Fatalf("welcome failed: %v: %s", err, output)
			}
			if test.code != 0 {
				var exit *exec.ExitError
				if !errors.As(err, &exit) || exit.ExitCode() != test.code {
					t.Fatalf("expected status %d, got %v: %s", test.code, err, output)
				}
			}
			if !utf8.Valid(output) || bytes.ContainsRune(output, '\x1b') {
				t.Fatalf("invalid plain welcome: %s", output)
			}
			for _, want := range test.want {
				if !strings.Contains(string(output), want) {
					t.Errorf("missing %q: %s", want, output)
				}
			}
			for _, absent := range test.absent {
				if strings.Contains(string(output), absent) {
					t.Errorf("unexpected %q: %s", absent, output)
				}
			}
			for _, line := range strings.Split(string(output), "\n") {
				columns := utf8.RuneCountInString(line) + strings.Count(line, "界")
				if columns > 80 || strings.TrimRight(line, " ") != line {
					t.Errorf("welcome line is %d columns or has trailing spaces: %q", columns, line)
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
	for _, name := range []string{"workspace.nix", "lmx.sh", "help.sh", "info.sh", "welcome.sh"} {
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
