package nixos

import (
	"archive/zip"
	"bytes"
	"embed"
	"encoding/json"
	"io"
	"io/fs"
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/limanix/client/internal/config"
	"github.com/limanix/client/internal/domain"
	"github.com/limanix/client/internal/nixos/catalog"
)

//go:embed testdata/evaluation
var evaluationFiles embed.FS

// Keep the selected release's platform and shared schemas, but no optional module
// implementation. Preset entries are inert stubs; the local and project modules
// exercise client-owned integration independently of catalog features.
func evaluationFixture(t *testing.T, arch domain.Architecture) (config.Config, *catalog.Catalog, string) {
	t.Helper()
	cfg := config.Default()
	cfg.Name = "eval-vm"
	cfg.Resources.Arch = arch
	cfg.User.Name, cfg.User.Home = "eval-user", "/home/eval-user"
	cfg.Network.Ports.TCP = []int{8080}
	cfg.Shell = config.Shell{
		Preset: config.PresetMinimal, Prompt: config.None, Editor: config.None, Multiplexer: config.None,
		Dotfiles: domain.OptionalHostPath(t.TempDir()),
	}
	cfg.Theme.Name = "fixture-theme"
	cfg.NixOS.Modules = []domain.ModuleID{"lmx:fixture", "third-party:capability"}
	cfg.NixOS.Packages = []domain.PackageName{"tree", "tree", "python3Packages.ipython", "terraform", "unstable:cowsay"}

	data, err := resources.ReadFile("resources/modules.zip")
	if err != nil {
		t.Fatal(err)
	}
	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{}
	for _, file := range archive.File {
		if file.FileInfo().IsDir() || (strings.HasPrefix(file.Name, "modules/") && !strings.HasPrefix(file.Name, "modules/_shared/")) {
			continue
		}
		input, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		contents, err := io.ReadAll(input)
		closeErr := input.Close()
		if err != nil {
			t.Fatal(err)
		}
		if closeErr != nil {
			t.Fatal(closeErr)
		}
		files[file.Name] = contents
	}
	files["modules/_shared/themes.toml"] = evaluationTestdata(t, "themes.toml")
	for _, choice := range cfg.Selection().Modules {
		if choice.ID.IsThirdParty() {
			continue
		}
		kind := "shell"
		declaration := []byte("{}\n")
		if choice.ID == "lmx:fixture" {
			kind = "modules"
			declaration = evaluationTestdata(t, "module.nix")
		}
		directory := path.Join("modules", kind, choice.ID.Selector())
		files[path.Join(directory, "module.toml")] = []byte("description = 'Synthetic catalog module.'\n")
		files[path.Join(directory, "default.nix")] = declaration
	}
	var packed bytes.Buffer
	writer := zip.NewWriter(&packed)
	if err := writer.SetComment(catalog.Repository); err != nil {
		t.Fatal(err)
	}
	for _, name := range slices.Sorted(maps.Keys(files)) {
		file, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := file.Write(files[name]); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	fixture, err := catalog.Open(packed.Bytes())
	if err != nil {
		t.Fatal(err)
	}

	project := t.TempDir()
	if err := os.WriteFile(filepath.Join(project, "default.nix"), evaluationTestdata(t, "project.nix"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "expected.json"), evaluationTestdata(t, "expected.json"), 0o600); err != nil {
		t.Fatal(err)
	}
	return cfg, fixture, project
}

func evaluationTestdata(t *testing.T, name string) []byte {
	t.Helper()
	data, err := evaluationFiles.ReadFile(path.Join("testdata", "evaluation", name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestEvaluationFixtureUsesSyntheticModules(t *testing.T) {
	cfg, fixture, project := evaluationFixture(t, domain.ARM64)
	if err := config.Validate(cfg); err != nil {
		t.Fatal(err)
	}
	var expected struct {
		Modules []domain.ModuleID `json:"modules"`
	}
	if err := json.Unmarshal(evaluationTestdata(t, "expected.json"), &expected); err != nil {
		t.Fatal(err)
	}
	var selected []domain.ModuleID
	for _, choice := range cfg.Selection().Modules {
		selected = append(selected, choice.ID)
	}
	if !slices.Equal(selected, expected.Modules) {
		t.Fatalf("fixture selection: got %v, want %v", selected, expected.Modules)
	}
	want := map[string]bool{}
	for _, choice := range cfg.Selection().Modules {
		if choice.ID.IsThirdParty() {
			continue
		}
		want[choice.ID.Selector()] = true
		module, err := fixture.Module(choice.ID.Selector())
		if err != nil {
			t.Fatal(err)
		}
		contents, err := fs.ReadFile(module, module.EntryPoint)
		if err != nil {
			t.Fatal(err)
		}
		expected := []byte("{}\n")
		if choice.ID == "lmx:fixture" {
			expected = evaluationTestdata(t, "module.nix")
		}
		if !bytes.Equal(contents, expected) {
			t.Fatalf("%s retains a real module implementation", choice.ID)
		}
	}
	if len(fixture.Modules()) != len(want) {
		t.Fatal("fixture retained unselected real modules")
	}
	if _, err := os.Stat(filepath.Join(project, "expected.json")); err != nil {
		t.Fatal(err)
	}
}
