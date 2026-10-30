package nixos

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/limanix/client/internal/config"
	"github.com/limanix/client/internal/domain"
	"github.com/limanix/client/internal/modules"
	"github.com/limanix/client/internal/nixos/catalog"
)

// testGeneration is the generation ID that tests pass to Prepare.
const testGeneration = "0123456789ab"

func TestRuntimeCarriesTheGeneration(t *testing.T) {
	flake, err := Prepare(config.Default(), filepath.Join(t.TempDir(), "runtime"), testGeneration, nil, 501, nil)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(flake, "runtime.json"))
	if err != nil {
		t.Fatal(err)
	}
	var runtime runtimeConfig
	if err = json.Unmarshal(data, &runtime); err != nil {
		t.Fatal(err)
	}
	if runtime.Generation != testGeneration || runtime.Theme.Name != "system" {
		t.Fatalf("generation or theme missing from runtime: %s", data)
	}
}

func TestRuntimeCarriesAuthorizedKeys(t *testing.T) {
	key := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIOwnTestKeyOfLimaForTheDevelopmentAccount lima"
	for _, test := range []struct {
		keys []string
		want string
	}{
		{[]string{key}, `"authorizedKeys": [` + "\n" + `      "` + key + `"`},
		{nil, `"authorizedKeys": []`},
	} {
		flake, err := Prepare(config.Default(), filepath.Join(t.TempDir(), "runtime"), testGeneration, nil, 501, test.keys)
		if err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(filepath.Join(flake, "runtime.json"))
		if err != nil || !bytes.Contains(data, []byte(test.want)) {
			t.Fatalf("runtime.json lacks %s: %s, %v", test.want, data, err)
		}
	}
	for _, key := range []string{"", key + "\nssh-ed25519 AAAAC3NzaC1lZDI1NTE5 other"} {
		if _, err := Prepare(config.Default(), filepath.Join(t.TempDir(), "runtime"), testGeneration, nil, 501, []string{key}); !errors.Is(err, ErrInvalidAuthorizedKey) {
			t.Fatalf("accepted authorized key %q: %v", key, err)
		}
	}
}

func TestRuntimeCarriesPackages(t *testing.T) {
	cfg := config.Default()
	cfg.NixOS.Packages = []domain.PackageName{"jq", "python3Packages.ipython", "jq"}
	flake, err := Prepare(cfg, filepath.Join(t.TempDir(), "runtime"), testGeneration, nil, 501, nil)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(flake, "runtime.json"))
	if err != nil {
		t.Fatal(err)
	}
	var runtime runtimeConfig
	if err = json.Unmarshal(data, &runtime); err != nil {
		t.Fatal(err)
	}
	want := []domain.PackageName{"gh", "jq", "python3Packages.ipython", "jq"}
	if !reflect.DeepEqual(runtime.Packages, want) {
		t.Fatalf("packages changed in runtime: %s", data)
	}

	cfg.NixOS.Packages = nil
	cfg.Shell = config.Shell{Preset: config.PresetMinimal, Prompt: config.None, Editor: config.None, Multiplexer: config.None}
	flake, err = Prepare(cfg, filepath.Join(t.TempDir(), "runtime"), testGeneration, nil, 501, nil)
	if err != nil {
		t.Fatal(err)
	}
	data, err = os.ReadFile(filepath.Join(flake, "runtime.json"))
	if err != nil || !bytes.Contains(data, []byte(`"packages": []`)) {
		t.Fatalf("empty packages must remain a JSON list: %s, %v", data, err)
	}
}

func TestPrepareRejectsAnInvalidGeneration(t *testing.T) {
	for _, generation := range []string{"", "0123456789AB", "0123456789abc", "../escape0000"} {
		directory := filepath.Join(t.TempDir(), "runtime")
		_, err := Prepare(config.Default(), directory, generation, nil, 501, nil)
		if !errors.Is(err, ErrInvalidGeneration) {
			t.Fatalf("generation %q: got %v", generation, err)
		}
		if _, err := os.Stat(directory); !os.IsNotExist(err) {
			t.Fatalf("generation %q staged files before validation", generation)
		}
	}
}

// TestLmxPinNamesAReleaseForEachSystem guards manual pin updates: lmx publishes one archive and
// its SHA-256 per system under the release tag.
func TestLmxPinNamesAReleaseForEachSystem(t *testing.T) {
	data, err := resources.ReadFile("resources/base/lmx.json")
	if err != nil {
		t.Fatal(err)
	}
	var pin struct {
		Version string `json:"version"`
		Systems map[string]struct {
			URL    string `json:"url"`
			SHA256 string `json:"sha256"`
		} `json:"systems"`
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&pin); err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`).MatchString(pin.Version) {
		t.Fatalf("pin version %q is not a release version", pin.Version)
	}
	if systems := slices.Sorted(maps.Keys(pin.Systems)); !slices.Equal(systems, []string{"aarch64-linux", "x86_64-linux"}) {
		t.Fatalf("pin systems: %v", systems)
	}
	digest := regexp.MustCompile(`^[0-9a-f]{64}$`)
	for system, archive := range pin.Systems {
		want := fmt.Sprintf("https://github.com/limanix/lmx/releases/download/v%[1]s/lmx-%[1]s-%[2]s.tar.gz", pin.Version, system)
		if archive.URL != want || !digest.MatchString(archive.SHA256) {
			t.Fatalf("%s: got %s %s, want %s and a SHA-256", system, archive.URL, archive.SHA256, want)
		}
	}
}

// TestImagePinNamesADiskForEachSystem guards the bundled manifest: a catalog release carries a disk and its SHA-256 per system.
func TestImagePinNamesADiskForEachSystem(t *testing.T) {
	data, err := resources.ReadFile("resources/image.json")
	if err != nil {
		t.Fatal(err)
	}
	image, err := catalog.ParseImage(data, "")
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`^v[1-9][0-9]*$`).MatchString(image.Version) {
		t.Fatalf("manifest version %q is not a catalog release", image.Version)
	}
	for _, architecture := range domain.Architectures() {
		disk, err := BaseImage(architecture)
		if err != nil || disk.Location == "" || disk.SHA256 == "" {
			t.Fatalf("%s: %+v, %v", architecture, disk, err)
		}
	}
}

func TestRuntimeMetadataPreservesSelectedModulesWithoutEnvironment(t *testing.T) {
	cfg := config.Default()
	cfg.Env["PRIVATE_TOKEN"] = "never-in-workspace-metadata"
	cfg.NixOS.Modules = []domain.ModuleID{"lmx:docker", "lmx:docker"}
	flake, err := Prepare(cfg, filepath.Join(t.TempDir(), "runtime"), testGeneration, []modules.Source{{ID: "lmx:docker"}, {ID: "lmx:docker"}}, 501, nil)
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
	selected := []domain.ModuleID{
		"lmx:cli-tools", "lmx:zsh", "lmx:lazygit", "lmx:yazi", "lmx:astronvim", "lmx:tmux", "lmx:docker", "lmx:docker",
	}
	if !reflect.DeepEqual(runtime.SelectedModules, selected) || strings.Contains(string(data), "never-in-workspace-metadata") {
		t.Fatalf("workspace metadata changed: %s", data)
	}
	for _, name := range []string{"lmx.nix", "lmx.json", "packages.nix", "packages-check.sh", "shell.nix", "workspace.nix"} {
		if _, err := os.Stat(filepath.Join(flake, name)); err != nil {
			t.Fatalf("base file %s missing: %v", name, err)
		}
	}
	cfg.NixOS.Modules = nil
	cfg.Shell = config.Shell{Preset: config.PresetMinimal, Prompt: config.None, Editor: config.None, Multiplexer: config.None}
	flake, err = Prepare(cfg, filepath.Join(t.TempDir(), "runtime"), testGeneration, nil, 501, nil)
	if err != nil {
		t.Fatal(err)
	}
	data, err = os.ReadFile(filepath.Join(flake, "runtime.json"))
	if err != nil || !bytes.Contains(data, []byte(`"selectedModules": [
    "lmx:cli-tools"
  ]`)) {
		t.Fatalf("a minimal shell selects only the CLI tools: %s, %v", data, err)
	}
}

func TestRuntimeCarriesGuestDiskPolicy(t *testing.T) {
	cfg := config.Default()
	cfg.Resources.Disk = domain.ByteSize(16 * domain.GiB)
	flake, err := Prepare(cfg, filepath.Join(t.TempDir(), "runtime"), testGeneration, nil, 501, nil)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(flake, "runtime.json"))
	if err != nil {
		t.Fatal(err)
	}
	var runtime struct {
		Disk struct {
			Bytes          int64  `json:"bytes"`
			CollectPercent uint64 `json:"collectPercent"`
			MinimumPercent uint64 `json:"minimumPercent"`
		} `json:"disk"`
	}
	if err = json.Unmarshal(data, &runtime); err != nil {
		t.Fatal(err)
	}
	if runtime.Disk.Bytes != 16*domain.GiB || runtime.Disk.CollectPercent != domain.DiskCollectPercent ||
		runtime.Disk.MinimumPercent != domain.DiskMinimumPercent {
		t.Fatalf("guest disk policy missing from runtime: %s", data)
	}
}

func TestRuntimeCarriesShellAndToolchains(t *testing.T) {
	read := func(cfg config.Config) runtimeConfig {
		t.Helper()
		flake, err := Prepare(cfg, filepath.Join(t.TempDir(), "runtime"), testGeneration, nil, 501, nil)
		if err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(filepath.Join(flake, "runtime.json"))
		if err != nil {
			t.Fatal(err)
		}
		var runtime runtimeConfig
		if err = json.Unmarshal(data, &runtime); err != nil {
			t.Fatal(err)
		}
		return runtime
	}

	runtime := read(config.Default())
	if runtime.Shell.Prompt == nil || *runtime.Shell.Prompt != "pure" || runtime.Shell.Dotfiles != nil || runtime.Toolchains.Rust != nil {
		t.Fatalf("defaults: %+v %+v", runtime.Shell, runtime.Toolchains)
	}

	cfg := config.Default()
	cfg.Shell = config.Shell{
		Preset: config.PresetMinimal, Prompt: config.None, Editor: config.None, Multiplexer: config.None,
		Dotfiles: domain.OptionalHostPath(t.TempDir()),
	}
	cfg.Toolchains = config.Toolchains{Rust: "nightly-2026-10-01", Go: "1.27"}
	runtime = read(cfg)
	if runtime.Shell.Prompt != nil || runtime.Shell.Dotfiles == nil || *runtime.Shell.Dotfiles != config.DotfilesTarget ||
		runtime.Toolchains.Rust == nil || *runtime.Toolchains.Rust != "nightly-2026-10-01" {
		t.Fatalf("minimal with dotfiles and Rust: %+v %+v", runtime.Shell, runtime.Toolchains)
	}
	if want := []domain.ModuleID{"lmx:cli-tools", "lmx:go-1.27", "lmx:rust"}; !reflect.DeepEqual(runtime.SelectedModules, want) {
		t.Fatalf("selected modules: %v", runtime.SelectedModules)
	}
}

func TestLocalLMXReplacesThePinnedRelease(t *testing.T) {
	flake := t.TempDir()
	if err := copyResource("resources/base", flake); err != nil {
		t.Fatal(err)
	}
	pinned, err := os.ReadFile(filepath.Join(flake, "lmx.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err = overlayLMX(fstest.MapFS{"resources/base/lmx.json": {Data: pinned}}, flake); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(filepath.Join(flake, "lmx.json")); err != nil || !bytes.Equal(data, pinned) {
		t.Fatalf("a build without local lmx changed the pin: %v", err)
	}

	local := fstest.MapFS{
		"resources/lmx/lmx.json":                       {Data: []byte(`{"version":"0.2.0","systems":{}}` + "\n")},
		"resources/lmx/lmx-0.2.0-aarch64-linux.tar.gz": {Data: []byte("archive")},
	}
	if err = overlayLMX(local, flake); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(filepath.Join(flake, "lmx.json")); err != nil || !bytes.Contains(data, []byte(`"0.2.0"`)) {
		t.Fatalf("local lmx.json not used: %s, %v", data, err)
	}
	if data, err := os.ReadFile(filepath.Join(flake, "lmx-0.2.0-aarch64-linux.tar.gz")); err != nil || string(data) != "archive" {
		t.Fatalf("local archive not copied: %v", err)
	}
}
