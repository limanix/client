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

// limanix-eval-shard-protocol: 1
//
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

	selectionNames := make([]string, 0, len(selections))
	for _, selection := range selections {
		selectionNames = append(selectionNames, selection.name)
	}
	moduleValue, moduleSet := os.LookupEnv("LIMANIX_EVAL_MODULES")
	moduleFilter, err := parseEvaluationFilter(moduleValue, moduleSet, selectionNames)
	if err != nil {
		t.Fatalf("LIMANIX_EVAL_MODULES: %v", err)
	}
	archNames := make([]string, 0, len(domain.Architectures()))
	for _, arch := range domain.Architectures() {
		archNames = append(archNames, string(arch))
	}
	archValue, archSet := os.LookupEnv("LIMANIX_EVAL_ARCH")
	archFilter, err := parseEvaluationFilter(archValue, archSet, archNames)
	if err != nil {
		t.Fatalf("LIMANIX_EVAL_ARCH: %v", err)
	}

	for _, arch := range domain.Architectures() {
		if !archFilter[string(arch)] {
			continue
		}
		for _, selection := range selections {
			if !moduleFilter[selection.name] {
				continue
			}
			t.Run(string(arch)+"/"+selection.name, func(t *testing.T) {
				cfg := config.Default()
				cfg.Name = "eval-vm"
				cfg.Resources.Arch = arch
				cfg.User.Name = "eval-user"
				cfg.User.Home = "/home/eval-user"
				cfg.Network.Ports.TCP = []int{8080}
				cfg.NixOS.Modules = selection.modules
				sources := make([]modules.Source, 0, len(selection.modules))
				for _, id := range selection.modules {
					source := modules.Source{ID: id}
					if selection.thirdParty {
						source.Path = t.TempDir()
						declaration := `{ config, lib, pkgs, ... }: {
  lmx.capabilities.languageSupport.languages.python.parsers = [ "python" ];
  # A port that the configuration declares too, and a range: the configuration of lmx lists each once.
  networking.firewall.allowedTCPPorts = [ 8080 ];
  networking.firewall.allowedTCPPortRanges = [ { from = 9000; to = 9002; } ];
  # NixOS accepts contextual strings and string-like store objects as packages.
  environment.systemPackages = [
    "${pkgs.hello}"
    { __toString = _: toString pkgs.hello; }
    (pkgs.hello // { meta = pkgs.hello.meta // { priority = 7; }; })
  ];
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
    {
      assertion = !config.nix.gc.automatic && config.nix.settings.auto-optimise-store
        && !config.documentation.doc.enable;
      message = "lmxd, not a timer, collects the guest store, and documentation outputs stay out.";
    }
    {
      assertion = config.nix.settings.min-free == 1073741824 && config.nix.settings.max-free == 2147483648
        && !(config.systemd.services ? limanix-store-guard) && !(config.systemd.timers ? limanix-store-guard)
        && lib.hasInfix "SystemMaxUse=512M" config.services.journald.extraConfig;
      message = "The platform must keep 10-20% of the default guest disk free during builds, lmxd checks between them, and the journal stays bounded.";
    }
    {
      assertion = builtins.any (package: (package.pname or "") == "lmx") config.environment.systemPackages;
      message = "Every guest must reach lmx, the Mac clipboard and sessions without a catalog module.";
    }
    {
      assertion =
        let
          # Tools are store paths, and fromJSON rejects their string context.
          text = builtins.unsafeDiscardStringContext config.environment.etc."lmx/config.json".text;
          settings = builtins.fromJSON text;
          tools = builtins.removeAttrs settings.tools [ "sudo" ];
        in
        builtins.attrNames settings == [ "disk" "generation" "health" "modules" "network" "schema" "session" "theme" "tools" "user" "vm" ]
        && builtins.attrNames settings.tools == [ "bash" "grep" "ionice" "ip" "journalctl" "nice" "nix_env" "nix_store" "nixos_rebuild" "sudo" "systemctl" "systemd_run" ]
        && settings.generation == "0123456789ab"
        && settings.user == { name = "eval-user"; home = "/home/eval-user"; uid = 501; gid = 100; }
        && builtins.elem "users" config.users.users.eval-user.extraGroups
        && settings.network.ports == { tcp = [ 22 8080 9000 9001 9002 ]; udp = [ ]; }
        && settings.health.units == [ "sshd.service" "lima-guestagent.service" "lmx.socket" ]
        && settings.theme.flavor == "mocha" && builtins.length (builtins.attrNames settings.theme.palette) == 26
        && settings.session.providers == config.limanix.session.providers
        && builtins.all (path: lib.hasPrefix "/nix/store/" path) (builtins.attrValues tools)
        && settings.tools.sudo == "/run/wrappers/bin/sudo";
      message = "lmx must receive the platform configuration of its generation.";
    }
    {
      assertion = builtins.elem "sockets.target" config.systemd.sockets.lmx.wantedBy
        && builtins.elem "multi-user.target" config.systemd.services.lmx.wantedBy
        && config.systemd.services.lmx.serviceConfig.Type == "notify" && !config.systemd.services.lmx.restartIfChanged
        && builtins.any (check: lib.hasPrefix "lmx-config-check" check.name) config.system.checks;
      message = "lmxd must run from boot, answer on its socket and accept the configuration of its generation.";
    }
    {
      assertion = !config.nix.channel.enable
        && builtins.elem "r /nix/var/nix/profiles/per-user/root/channels-*-link" config.systemd.tmpfiles.rules;
      message = "The base image's channel must not remain a garbage-collector root.";
    }
  ];
}`
						if err := os.WriteFile(filepath.Join(source.Path, "default.nix"), []byte(declaration), 0o600); err != nil {
							t.Fatal(err)
						}
					}
					sources = append(sources, source)
				}

				flake, err := Prepare(cfg, filepath.Join(t.TempDir(), "generation"), testGeneration, sources, 501)
				if err != nil {
					t.Fatal(err)
				}
				command := exec.CommandContext(
					t.Context(), nix,
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
				if selection.thirdParty {
					// The host starts the transient lmxd from this output; one case keeps the cost low.
					system := map[domain.Architecture]string{"arm64": "aarch64-linux", "amd64": "x86_64-linux"}[arch]
					lmxCommand := exec.CommandContext(
						t.Context(), nix,
						"eval", "--raw", "--show-trace",
						"--extra-experimental-features", "nix-command flakes",
						"--no-update-lock-file", "--no-write-lock-file",
						"--option", "allow-import-from-derivation", "false",
						"path:.#packages."+system+".lmx.drvPath",
					)
					lmxCommand.Dir = flake
					diagnostics.Reset()
					lmxCommand.Stderr = &diagnostics
					output, err := lmxCommand.Output()
					if err != nil || !strings.HasSuffix(strings.TrimSpace(string(output)), ".drv") {
						t.Fatalf("evaluate the lmx package: %v\n%s\n%s", err, output, diagnostics.String())
					}
				}
			})
		}
	}
	if !t.Failed() {
		t.Logf("Evaluated %d generated-flake cases across %d architecture(s).", len(moduleFilter)*len(archFilter), len(archFilter))
	}
}
