package nixos

import (
	"maps"
	"slices"
	"testing"

	"github.com/limanix/client/internal/config"
	"github.com/limanix/client/internal/domain"
	"github.com/limanix/client/internal/nixos/catalog"
)

// evaluationCase is one configuration that TestGeneratedFlakeEvaluation evaluates. .github/scripts/plan_eval.py
// derives the same names from a catalog tag; change both together and bump the shard protocol.
type evaluationCase struct {
	name       string
	configure  func(*config.Config)
	thirdParty bool
}

// workbenchModules are the modules of the workbench case, which needs all of them.
var workbenchModules = []string{"docker", "gcloud", "claude", "posting"}

func minimalShell(cfg *config.Config) {
	cfg.Shell = config.Shell{Preset: config.PresetMinimal, Prompt: config.None, Editor: config.None, Multiplexer: config.None}
}

// setToolchain sets the [toolchains] key of a catalog toolchain module.
func setToolchain(cfg *config.Config, key, value string) {
	switch key {
	case "go":
		cfg.Toolchains.Go = domain.ToolchainLine(value)
	case "rust":
		cfg.Toolchains.Rust = domain.RustToolchain(value)
	case "python":
		cfg.Toolchains.Python = domain.ToolchainLine(value)
	case "nodejs":
		cfg.Toolchains.Nodejs = domain.ToolchainLine(value)
	}
}

// evaluationCases covers each shell preset and switch, each module of kind modules alone, each toolchain value that the
// client can name, a project module, and one workbench that combines them.
func evaluationCases(entries map[string]catalog.Entry) []evaluationCase {
	cases := []evaluationCase{
		{name: "empty", configure: minimalShell},
		{name: "third-party-capability", configure: minimalShell, thirdParty: true},
		{name: "shell-terminal"},
		{name: "shell-terminal-starship", configure: func(cfg *config.Config) {
			cfg.Shell.Prompt, cfg.Shell.Editor = "starship", "neovim"
		}},
		{name: "shell-terminal-bare", configure: func(cfg *config.Config) {
			cfg.Shell.Prompt, cfg.Shell.Editor, cfg.Shell.Multiplexer = config.None, config.None, config.None
		}},
		{name: "shell-minimal-tools", configure: func(cfg *config.Config) {
			minimalShell(cfg)
			cfg.Shell.Editor, cfg.Shell.Multiplexer = "astronvim", "tmux"
		}},
	}

	lined := map[string]bool{}
	for _, entry := range entries {
		lined[entry.Module] = lined[entry.Module] || entry.Line != ""
	}

	newest := map[string]string{}
	for _, selector := range slices.Sorted(maps.Keys(entries)) {
		entry := entries[selector]
		switch entry.Kind {
		case catalog.KindModules:
			id := domain.ModuleID("lmx:" + selector)
			cases = append(cases, evaluationCase{name: selector, configure: func(cfg *config.Config) {
				minimalShell(cfg)
				cfg.NixOS.Modules = []domain.ModuleID{id}
			}})
		case catalog.KindToolchains:
			// The client names a line of a toolchain that has lines, and a channel of one without.
			key, _ := config.ToolchainKey(entry.Module)
			value := entry.Line
			if !lined[entry.Module] {
				value = "stable"
			} else if value == "" {
				continue
			}
			if previous, seen := newest[key]; !seen || compareVersions(value, previous) > 0 {
				newest[key] = value
			}
			cases = append(cases, evaluationCase{name: selector, configure: func(cfg *config.Config) {
				minimalShell(cfg)
				setToolchain(cfg, key, value)
			}})
		}
	}

	if slices.ContainsFunc(workbenchModules, func(name string) bool { return entries[name].Kind != catalog.KindModules }) {
		return cases
	}

	return append(cases, evaluationCase{name: "workbench", configure: func(cfg *config.Config) {
		for key, value := range newest {
			setToolchain(cfg, key, value)
		}
		for _, name := range workbenchModules {
			cfg.NixOS.Modules = append(cfg.NixOS.Modules, domain.ModuleID("lmx:"+name))
		}
		cfg.NixOS.Packages = []domain.PackageName{"kubectl", "unstable:kubernetes-helm"}
	}})
}

// compareVersions orders lines such as 1.9 and 1.10 by their numeric components; channels compare equal.
func compareVersions(a, b string) int {
	parse := func(value string) []int {
		var result []int
		number, digits := 0, false
		for _, character := range value + "." {
			switch {
			case character >= '0' && character <= '9':
				number, digits = number*10+int(character-'0'), true
			case character == '.' && digits:
				result, number, digits = append(result, number), 0, false
			default:
				return nil
			}
		}
		return result
	}

	return slices.Compare(parse(a), parse(b))
}

func TestEvaluationCasesAreUniqueAndCoverTheCatalog(t *testing.T) {
	entries, err := SystemModules()
	if err != nil {
		t.Fatal(err)
	}

	cases := evaluationCases(entries)
	names := map[string]bool{}
	for _, evaluation := range cases {
		if names[evaluation.name] {
			t.Fatalf("duplicate case %q", evaluation.name)
		}
		names[evaluation.name] = true
	}
	lined := map[string]bool{}
	for _, entry := range entries {
		lined[entry.Module] = lined[entry.Module] || entry.Line != ""
	}
	for selector, entry := range entries {
		covered := entry.Kind == catalog.KindShell || (entry.Kind == catalog.KindToolchains && lined[entry.Module] && entry.Line == "")
		if !covered && !names[selector] {
			t.Fatalf("no case evaluates %s", selector)
		}
	}
	if !names["workbench"] {
		t.Fatal("the bundled catalog lacks the workbench modules")
	}
	if compareVersions("1.10", "1.9") <= 0 || compareVersions("26", "24") <= 0 {
		t.Fatal("lines must compare numerically")
	}
}
