package config

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/limanix/client/internal/domain"
)

func TestOmittedShellIsTheTerminalPreset(t *testing.T) {
	parsed, err := Parse([]byte("name = 'box'\n"))
	if err != nil {
		t.Fatal(err)
	}
	want := Shell{Preset: PresetTerminal, Prompt: "pure", Editor: "astronvim", Multiplexer: "tmux"}
	if parsed.Shell != want || parsed.Toolchains != (Toolchains{}) {
		t.Fatalf("defaults: %#v %#v", parsed.Shell, parsed.Toolchains)
	}
}

func TestMinimalShellSwitchesOffWhatItDoesNotName(t *testing.T) {
	for document, want := range map[string]Shell{
		"[shell]\npreset = 'minimal'\n": {Preset: PresetMinimal, Prompt: None, Editor: None, Multiplexer: None},
		"[shell]\npreset = 'minimal'\neditor = 'neovim'\nmultiplexer = 'tmux'\n": {
			Preset: PresetMinimal, Prompt: None, Editor: "neovim", Multiplexer: "tmux",
		},
		"[shell]\nprompt = 'starship'\neditor = 'none'\n": {
			Preset: PresetTerminal, Prompt: "starship", Editor: None, Multiplexer: "tmux",
		},
	} {
		parsed, err := Parse([]byte(document))
		if err != nil || parsed.Shell != want {
			t.Fatalf("%q: %#v, %v", document, parsed.Shell, err)
		}
		rendered, err := Render(parsed)
		if err != nil {
			t.Fatal(err)
		}
		if restored, err := Parse(rendered); err != nil || restored.Shell != want {
			t.Fatalf("%q: round trip %#v, %v", document, restored.Shell, err)
		}
	}
}

func TestShellAndToolchainDiagnostics(t *testing.T) {
	for document, want := range map[string]string{
		"[shell]\npreset = 'fancy'\n":                               "shell.preset: expected one of terminal, minimal",
		"[shell]\nprompt = 'Oh My Posh'\n":                          "shell.prompt: expected none or the name of a catalog prompt",
		"[shell]\neditor = 'vim'\n":                                 "shell.editor: expected one of astronvim, neovim, none",
		"[shell]\ndotfiles = 3\n":                                   "shell.dotfiles: expected a string",
		"[toolchains]\ngo = '1.x'\n":                                "toolchains.go: expected a version line",
		"[toolchains]\nrust = 'latest'\n":                           "toolchains.rust: expected stable, beta, nightly",
		"[toolchains]\nrust = 'stable-aarch64-unknown-linux-gnu'\n": "toolchains.rust: expected stable",
		"[toolchains]\nzig = '0.15'\n":                              "toolchains.zig: unknown field",
		"[toolchains]\nmaven = '3'\n":                               "toolchains.maven: needs a JDK",
		"[toolchains]\ngradle = '9'\nmaven = '3'\n":                 "toolchains.maven: needs a JDK",
		"[toolchains]\ngradle = '9'\n":                              "toolchains.gradle: needs a JDK",
	} {
		_, err := Parse([]byte(document))
		if err == nil || !strings.HasPrefix(err.Error(), want) {
			t.Fatalf("%q: got %v, want %s", document, err, want)
		}
	}

	parsed, err := Parse([]byte("[toolchains]\ngo = '1.27'\nrust = 'nightly-2026-10-01'\npython = '3.12'\nnodejs = '24'\njava = '17'\nmaven = '3'\ngradle = '8'\ngcc = '14'\nclang = '21'\n"))
	want := Toolchains{Go: "1.27", Rust: "nightly-2026-10-01", Python: "3.12", Nodejs: "24", Java: "17", Maven: "3", Gradle: "8", Gcc: "14", Clang: "21"}
	if err != nil || parsed.Toolchains != want {
		t.Fatalf("toolchains: %#v, %v", parsed.Toolchains, err)
	}
	for _, value := range []string{"stable", "beta", "nightly", "1.90", "1.90.0", "beta-2026-10-01"} {
		if _, err := domain.NewRustToolchain(value); err != nil {
			t.Fatalf("%s refused: %v", value, err)
		}
	}
}

func TestUnstablePackages(t *testing.T) {
	parsed, err := Parse([]byte("[nixos]\npackages = ['jq', 'unstable:codex', 'unstable:python3Packages.ipython']\n"))
	if err != nil {
		t.Fatal(err)
	}
	if !parsed.NixOS.Packages[1].Unstable() || parsed.NixOS.Packages[1].Attribute() != "codex" || parsed.NixOS.Packages[0].Unstable() {
		t.Fatalf("package sources: %#v", parsed.NixOS.Packages)
	}
	for _, name := range []string{"nur:foo", "unstable:", "unstable:unstable:jq", "Unstable:jq"} {
		if _, err := domain.NewPackageName(name); err == nil {
			t.Fatalf("%q accepted", name)
		}
	}
	if _, err := domain.NewPackageName("nur:foo"); !errors.Is(err, domain.ErrUnsupportedPackageSource) {
		t.Fatalf("unknown source: %v", err)
	}
}

func TestSelectionOfTheShellAndToolchains(t *testing.T) {
	cfg := Default()
	cfg.Toolchains = Toolchains{Go: "1.27", Rust: "stable", Python: "3.14", Nodejs: "26", Java: "21", Maven: "3", Gradle: "9", Gcc: "15", Clang: "22"}
	cfg.NixOS.Modules = []domain.ModuleID{"lmx:docker", "third-party:custom"}
	cfg.NixOS.Packages = []domain.PackageName{"kubectl"}

	selection := cfg.Selection()
	want := []Choice{
		{ID: "lmx:cli-tools", Field: "shell.preset"},
		{ID: "lmx:zsh", Field: "shell.preset"},
		{ID: "lmx:lazygit", Field: "shell.preset"},
		{ID: "lmx:yazi", Field: "shell.preset"},
		{ID: "lmx:pure", Field: "shell.prompt", Shell: "zsh"},
		{ID: "lmx:astronvim", Field: "shell.editor"},
		{ID: "lmx:tmux", Field: "shell.multiplexer"},
		{ID: "lmx:go-1.27", Field: "toolchains.go"},
		{ID: "lmx:rust", Field: "toolchains.rust"},
		{ID: "lmx:python-3.14", Field: "toolchains.python"},
		{ID: "lmx:nodejs-26", Field: "toolchains.nodejs"},
		{ID: "lmx:java-21", Field: "toolchains.java"},
		{ID: "lmx:maven-3", Field: "toolchains.maven"},
		{ID: "lmx:gradle-9", Field: "toolchains.gradle"},
		{ID: "lmx:gcc-15", Field: "toolchains.gcc"},
		{ID: "lmx:clang-22", Field: "toolchains.clang"},
		{ID: "lmx:docker", Field: "nixos.modules[0]"},
		{ID: "third-party:custom", Field: "nixos.modules[1]"},
	}
	if !reflect.DeepEqual(selection.Modules, want) {
		t.Fatalf("modules: %#v", selection.Modules)
	}
	if !reflect.DeepEqual(selection.Packages, []domain.PackageName{"gh", "kubectl"}) {
		t.Fatalf("packages: %#v", selection.Packages)
	}

	cfg.Shell = Shell{Preset: PresetMinimal, Prompt: None, Editor: "neovim", Multiplexer: None}
	cfg.Toolchains = Toolchains{}
	cfg.NixOS.Modules = nil
	selection = cfg.Selection()
	want = []Choice{{ID: "lmx:cli-tools", Field: "shell.preset"}, {ID: "lmx:neovim", Field: "shell.editor"}}
	if !reflect.DeepEqual(selection.Modules, want) || !reflect.DeepEqual(selection.Packages, []domain.PackageName{"kubectl"}) {
		t.Fatalf("minimal: %#v %#v", selection.Modules, selection.Packages)
	}

	// A listed GitHub CLI, from either source, replaces the preset's.
	cfg = Default()
	cfg.NixOS.Packages = []domain.PackageName{"unstable:gh"}
	if packages := cfg.Selection().Packages; !reflect.DeepEqual(packages, []domain.PackageName{"unstable:gh"}) {
		t.Fatalf("listed gh: %#v", packages)
	}
}

func TestExampleShowsOptionalValuesCommentedOut(t *testing.T) {
	example, err := RenderExample()
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range []string{
		"[shell]\n",
		"preset = \"terminal\"\n",
		"# dotfiles = \"~/dotfiles\"\n",
		"[toolchains]\n",
		"# go = \"1.27\"\n",
		"# rust = \"stable\"\n",
	} {
		if !strings.Contains(string(example), line) {
			t.Fatalf("example lacks %q:\n%s", line, example)
		}
	}
}

func TestDotfilesResolveBesideTheConfiguration(t *testing.T) {
	directory := t.TempDir()
	if err := os.Mkdir(filepath.Join(directory, "dotfiles"), 0o700); err != nil {
		t.Fatal(err)
	}
	filename := filepath.Join(directory, "limanix.toml")
	if err := os.WriteFile(filename, []byte("[shell]\ndotfiles = 'dotfiles'\n[[mounts]]\nsource = '.'\ntarget = '/workspace'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(filename)
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := filepath.EvalSymlinks(filepath.Join(directory, "dotfiles"))
	if err != nil || string(loaded.Shell.Dotfiles) != resolved {
		t.Fatalf("dotfiles %q, want %q: %v", loaded.Shell.Dotfiles, resolved, err)
	}

	if _, err := Parse([]byte("[[mounts]]\nsource = '/tmp'\ntarget = '/mnt/dotfiles/notes'\n")); err == nil ||
		!strings.Contains(err.Error(), "reserved for the guest system") {
		t.Fatalf("mount over the dotfiles accepted: %v", err)
	}
}
