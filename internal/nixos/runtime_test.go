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

	"github.com/limanix/client/internal/config"
	"github.com/limanix/client/internal/domain"
	"github.com/limanix/client/internal/modules"
)

// testGeneration is the generation ID that tests pass to Prepare.
const testGeneration = "0123456789ab"

func TestRuntimeCarriesTheGeneration(t *testing.T) {
	flake, err := Prepare(config.Default(), filepath.Join(t.TempDir(), "runtime"), testGeneration, nil, 501)
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
	if runtime.Generation != testGeneration || runtime.Theme.Flavor != "mocha" {
		t.Fatalf("generation or theme missing from runtime: %s", data)
	}
}

func TestPrepareRejectsAnInvalidGeneration(t *testing.T) {
	for _, generation := range []string{"", "0123456789AB", "0123456789abc", "../escape0000"} {
		directory := filepath.Join(t.TempDir(), "runtime")
		_, err := Prepare(config.Default(), directory, generation, nil, 501)
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

func TestRuntimeMetadataPreservesSelectedModulesWithoutEnvironment(t *testing.T) {
	cfg := config.Default()
	cfg.Env["PRIVATE_TOKEN"] = "never-in-workspace-metadata"
	cfg.NixOS.Modules = []domain.ModuleID{"lmx:git", "lmx:git"}
	flake, err := Prepare(cfg, filepath.Join(t.TempDir(), "runtime"), testGeneration, []modules.Source{{ID: "lmx:git"}, {ID: "lmx:git"}}, 501)
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
	for _, name := range []string{"lmx.nix", "lmx.json", "workspace.nix"} {
		if _, err := os.Stat(filepath.Join(flake, name)); err != nil {
			t.Fatalf("base file %s missing: %v", name, err)
		}
	}
	cfg.NixOS.Modules = nil
	flake, err = Prepare(cfg, filepath.Join(t.TempDir(), "runtime"), testGeneration, nil, 501)
	if err != nil {
		t.Fatal(err)
	}
	data, err = os.ReadFile(filepath.Join(flake, "runtime.json"))
	if err != nil || !bytes.Contains(data, []byte(`"selectedModules": []`)) {
		t.Fatalf("empty modules must remain a JSON list: %s, %v", data, err)
	}
}

func TestRuntimeCarriesGuestDiskPolicy(t *testing.T) {
	cfg := config.Default()
	cfg.Resources.Disk = domain.ByteSize(16 * domain.GiB)
	flake, err := Prepare(cfg, filepath.Join(t.TempDir(), "runtime"), testGeneration, nil, 501)
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
