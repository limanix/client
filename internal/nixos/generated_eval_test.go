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

// TestGeneratedFlakeEvaluation evaluates Prepare's complete output, including the
// client platform, runtime mapping and embedded catalog; it does not build or boot a VM.
// Run through task ci/nixos-eval so the selected catalog and Nix are available.
func TestGeneratedFlakeEvaluation(t *testing.T) {
	nix, err := exec.LookPath("nix")
	if err != nil {
		t.Fatalf("Nix is required for generated-flake evaluation: %v", err)
	}

	catalog, err := SystemModules()
	if err != nil {
		t.Fatal(err)
	}
	type selection struct {
		name       string
		modules    []domain.ModuleID
		thirdParty bool
	}
	selections := []selection{
		{name: "empty"},
		{name: "third-party-capability", modules: []domain.ModuleID{"third-party:capability"}, thirdParty: true},
	}
	for _, selector := range slices.Sorted(maps.Keys(catalog)) {
		selections = append(selections, selection{
			name:    selector,
			modules: []domain.ModuleID{domain.ModuleID("lmx:" + selector)},
		})
	}
	integration := []string{"console", "astronvim-6", "go"}
	if !slices.ContainsFunc(integration, func(selector string) bool {
		_, available := catalog[selector]
		return !available
	}) {
		ids := make([]domain.ModuleID, 0, len(integration))
		for _, selector := range integration {
			ids = append(ids, domain.ModuleID("lmx:"+selector))
		}
		selections = append(selections, selection{name: "console-version", modules: ids})
	}

	for _, arch := range domain.Architectures() {
		for _, selection := range selections {
			t.Run(string(arch)+"/"+selection.name, func(t *testing.T) {
				cfg := config.Default()
				cfg.Name = "eval-vm"
				cfg.Resources.Arch = arch
				cfg.User.Name = "eval-user"
				cfg.User.Home = "/home/eval-user"
				cfg.NixOS.Modules = selection.modules
				sources := make([]modules.Source, 0, len(selection.modules))
				for _, id := range selection.modules {
					source := modules.Source{ID: id}
					if selection.thirdParty {
						source.Path = t.TempDir()
						declaration := `{ config, pkgs, ... }: {
  lmx.capabilities.editor.languages.python.parsers = [ "python" ];
  assertions = [
    {
      assertion = config.limanix.user.name == "eval-user"
        && config.users.users.eval-user.uid == 501
        && config.limanix.user.home == "/home/eval-user";
      message = "The client must populate the public guest identity.";
    }
    {
      assertion = builtins.elem pkgs.ghostty.terminfo config.environment.systemPackages;
      message = "Terminal compatibility must not require a catalog module.";
    }
    {
      assertion = !config.programs.neovim.enable;
      message = "Declaring a language must not activate an editor.";
    }
  ];
}`
						if err := os.WriteFile(filepath.Join(source.Path, "default.nix"), []byte(declaration), 0o600); err != nil {
							t.Fatal(err)
						}
					}
					sources = append(sources, source)
				}

				flake, err := Prepare(cfg, filepath.Join(t.TempDir(), "generation"), sources, 501)
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
					t.Fatalf("evaluate generated flake: %v\n%s\n%s", err, output, diagnostics.String())
				}
				derivation := strings.TrimSpace(string(output))
				if !strings.HasPrefix(derivation, "/nix/store/") || !strings.HasSuffix(derivation, ".drv") {
					t.Fatalf("evaluation did not return a system derivation: %s", output)
				}
				t.Logf("Evaluated %s", derivation)
			})
		}
	}
}
