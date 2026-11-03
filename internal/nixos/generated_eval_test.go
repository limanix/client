//go:build nix_eval

package nixos

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/limanix/client/internal/config"
	"github.com/limanix/client/internal/domain"
	"github.com/limanix/client/internal/modules"
	"github.com/limanix/client/internal/nixos/catalog"
)

const testAuthorizedKey = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIEvaluationKeyOfTheDevelopmentAccount lima"

// TestGeneratedFlakeEvaluation checks the real client defaults and client-owned
// integration mechanisms. Catalog feature tests belong to the modules repository.
func TestGeneratedFlakeEvaluation(t *testing.T) {
	nix, err := exec.LookPath("nix")
	if err != nil {
		t.Fatalf("Nix is required for generated-flake evaluation: %v", err)
	}
	value, supplied := os.LookupEnv("LIMANIX_EVAL_ARCH")
	architectures, err := evaluationArchitectures(value, supplied)
	if err != nil {
		t.Fatalf("LIMANIX_EVAL_ARCH: %v", err)
	}

	for _, arch := range architectures {
		t.Run(string(arch), func(t *testing.T) {
			t.Run("default VM", func(t *testing.T) {
				cfg := config.Default()
				cfg.Resources.Arch = arch
				flake := prepareEvaluation(t, cfg, "")
				if _, err := evaluateDerivation(t, nix, flake, "nixosConfigurations.runtime.config.system.build.toplevel.drvPath"); err != nil {
					t.Fatal(err)
				}
			})

			t.Run("client fixtures", func(t *testing.T) {
				cfg, fixture, project := evaluationFixture(t, arch)
				useEvaluationCatalog(t, fixture)
				flake := prepareEvaluation(t, cfg, project)
				if _, err := evaluateDerivation(t, nix, flake, "nixosConfigurations.runtime.config.system.build.toplevel.drvPath"); err != nil {
					t.Fatal(err)
				}
				projectSummary, err := evaluateAttribute(t, nix, flake, "nixosConfigurations.runtime.config.limanix.help.project.summary")
				if err != nil || projectSummary != "Tools of the project." {
					t.Fatalf("project module was not evaluated: summary=%q, error=%v", projectSummary, err)
				}

				system, err := arch.LimaArch()
				if err != nil {
					t.Fatal(err)
				}
				if _, err := evaluateDerivation(t, nix, flake, "packages."+system+"-linux.lmx.drvPath"); err != nil {
					t.Fatalf("evaluate this generation's lmx package: %v", err)
				}

				cfg.NixOS.Modules = nil
				cfg.NixOS.Packages = []domain.PackageName{"limanix-no-such-package"}
				missing := prepareEvaluation(t, cfg, "")
				_, err = evaluateDerivation(t, nix, missing, "nixosConfigurations.runtime.config.system.build.toplevel.drvPath")
				expected := `nixos.packages: "limanix-no-such-package" is not a package in nixpkgs`
				if err == nil || !strings.Contains(err.Error(), expected) {
					t.Fatalf("unknown package must fail with %q: %v", expected, err)
				}
			})
		})
	}
}

func prepareEvaluation(t *testing.T, cfg config.Config, project string) string {
	t.Helper()
	if err := config.Validate(cfg); err != nil {
		t.Fatalf("invalid evaluation fixture: %v", err)
	}
	var sources []modules.Source
	for _, choice := range cfg.Selection().Modules {
		source := modules.Source{ID: choice.ID}
		if choice.ID.IsThirdParty() {
			source.Path = project
		}
		sources = append(sources, source)
	}
	flake, err := Prepare(cfg, filepath.Join(t.TempDir(), "generation"), testGeneration, sources, 501, []string{testAuthorizedKey})
	if err != nil {
		t.Fatal(err)
	}
	return flake
}

func evaluateDerivation(t *testing.T, nix, flake, attribute string) (string, error) {
	t.Helper()
	value, err := evaluateAttribute(t, nix, flake, attribute)
	if err != nil {
		return "", err
	}
	derivation := strings.TrimSpace(value)
	if !strings.HasPrefix(derivation, "/nix/store/") || !strings.HasSuffix(derivation, ".drv") {
		return "", fmt.Errorf("%s did not return a derivation: %s", attribute, value)
	}
	t.Logf("Evaluated %s", derivation)
	return derivation, nil
}

func evaluateAttribute(t *testing.T, nix, flake, attribute string) (string, error) {
	t.Helper()
	command := exec.CommandContext(t.Context(), nix,
		"eval", "--raw", "--show-trace",
		"--extra-experimental-features", "nix-command flakes",
		"--no-update-lock-file", "--no-write-lock-file",
		"--option", "allow-import-from-derivation", "false",
		"path:.#"+attribute,
	)
	command.Dir = flake
	var diagnostics bytes.Buffer
	command.Stderr = &diagnostics
	output, err := command.Output()
	if err != nil {
		return "", fmt.Errorf("evaluate %s: %w\n%s\n%s", attribute, err, output, diagnostics.String())
	}
	return string(output), nil
}

// These tests are sequential: replacing the catalog must not overlap another test.
func useEvaluationCatalog(t *testing.T, fixture *catalog.Catalog) {
	t.Helper()
	original := systemCatalog
	systemCatalog = func() (*catalog.Catalog, error) { return fixture, nil }
	t.Cleanup(func() { systemCatalog = original })
}
