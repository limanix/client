//go:build nix_eval

package nixos

import (
	"bytes"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/limanix/client/internal/config"
	"github.com/limanix/client/internal/domain"
	"github.com/limanix/client/internal/modules"
)

// TestCatalogCompatibilityEvaluation composes every public catalog entry with
// this client's generated platform. It does not test TOML selection or module behavior.
// Keep this file standalone: release checks also inject it into older client sources.
func TestCatalogCompatibilityEvaluation(t *testing.T) {
	nix, err := exec.LookPath("nix")
	if err != nil {
		t.Fatalf("Nix is required for catalog compatibility evaluation: %v", err)
	}
	entries, err := SystemModules()
	if err != nil {
		t.Fatal(err)
	}
	selectors := slices.Sorted(maps.Keys(entries))
	if len(selectors) == 0 {
		t.Fatal("the catalog has no public entry points")
	}

	for _, arch := range catalogCompatibilityArchitectures(t) {
		for _, selector := range selectors {
			t.Run(string(arch)+"/"+selector, func(t *testing.T) {
				cfg := config.Default()
				cfg.Resources.Arch = arch
				cfg.Shell = config.Shell{
					Preset: config.PresetMinimal, Prompt: config.None, Editor: config.None, Multiplexer: config.None,
				}
				sources := catalogCompatibilitySources(cfg, domain.ModuleID("lmx:"+selector))
				flake, err := Prepare(cfg, filepath.Join(t.TempDir(), "generation"), "0123456789ab", sources, 501, nil)
				if err != nil {
					t.Fatal(err)
				}
				command := exec.CommandContext(t.Context(), nix,
					"eval", "--raw", "--show-trace",
					"--extra-experimental-features", "nix-command flakes",
					"--no-update-lock-file", "--no-write-lock-file",
					"--option", "allow-import-from-derivation", "false",
					"path:.#nixosConfigurations.runtime.config.system.build.toplevel.drvPath",
				)
				command.Dir = flake
				var diagnostics bytes.Buffer
				command.Stderr = &diagnostics
				output, err := command.Output()
				if err != nil {
					t.Fatalf("evaluate catalog entry: %v\n%s\n%s", err, output, diagnostics.String())
				}
				derivation := strings.TrimSpace(string(output))
				if !strings.HasPrefix(derivation, "/nix/store/") || !strings.HasSuffix(derivation, ".drv") {
					t.Fatalf("catalog entry did not produce a system derivation: %s", output)
				}
				t.Logf("Evaluated %s", derivation)
			})
		}
	}
}

func catalogCompatibilitySources(cfg config.Config, target domain.ModuleID) []modules.Source {
	var sources []modules.Source
	seen := map[domain.ModuleID]bool{}
	for _, choice := range cfg.Selection().Modules {
		if !seen[choice.ID] {
			sources = append(sources, modules.Source{ID: choice.ID})
			seen[choice.ID] = true
		}
	}
	if !seen[target] {
		sources = append(sources, modules.Source{ID: target})
	}
	return sources
}

func catalogCompatibilityArchitectures(t *testing.T) []domain.Architecture {
	t.Helper()
	available := domain.Architectures()
	value, supplied := os.LookupEnv("LIMANIX_EVAL_ARCH")
	if !supplied {
		return available
	}
	var selected []domain.Architecture
	for _, name := range strings.Fields(value) {
		arch := domain.Architecture(name)
		if !slices.Contains(available, arch) {
			t.Fatalf("LIMANIX_EVAL_ARCH: unknown architecture %q", name)
		}
		if !slices.Contains(selected, arch) {
			selected = append(selected, arch)
		}
	}
	if len(selected) == 0 {
		t.Fatal("LIMANIX_EVAL_ARCH: select at least one guest architecture")
	}
	return selected
}
