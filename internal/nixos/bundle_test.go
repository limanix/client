package nixos

import (
	"encoding/json"
	"io/fs"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/limanix/client/internal/config"
	"github.com/limanix/client/internal/domain"
	"github.com/limanix/client/internal/modules"
)

func TestEmbeddedModulesAndPinnedBaseCopied(t *testing.T) {
	cfg := config.Default()
	metadata, err := SystemModules()
	if err != nil {
		t.Fatal(err)
	}

	var sources []modules.Source
	for _, name := range slices.Sorted(maps.Keys(metadata)) {
		sources = append(sources, modules.Source{ID: domain.ModuleID("lmx:" + name)})
	}
	sources = append(sources, sources...)
	for _, source := range sources {
		cfg.NixOS.Modules = append(cfg.NixOS.Modules, source.ID)
	}

	flake, err := Prepare(cfg, filepath.Join(t.TempDir(), "runtime"), testGeneration, sources, 501)
	if err != nil {
		t.Fatal(err)
	}
	assertBundledInterface(t, flake)
	expectedImports := publicDeclarationImports(t)
	for _, source := range sources {
		files, err := systemModule(source.ID)
		if err != nil {
			t.Fatal(err)
		}

		original, err := fs.ReadFile(files, files.EntryPoint)
		if err != nil {
			t.Fatal(err)
		}
		entry := filepath.Join("modules", "lmx", files.EntryPoint)
		expectedImports = append(expectedImports, filepath.ToSlash(entry))
		copied, err := os.ReadFile(filepath.Join(flake, entry))
		if err != nil {
			t.Fatal(err)
		}
		if string(copied) != string(original) {
			t.Fatal("embedded module changed while copying")
		}
	}
	runtimeData, err := os.ReadFile(filepath.Join(flake, "runtime.json"))
	if err != nil {
		t.Fatal(err)
	}
	var runtime runtimeConfig
	if err := json.Unmarshal(runtimeData, &runtime); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(runtime.Modules, expectedImports) {
		t.Fatalf("selected entry points: got %v, want %v", runtime.Modules, expectedImports)
	}
	lock, err := os.ReadFile(filepath.Join(flake, "flake.lock"))
	if err != nil {
		t.Fatal(err)
	}
	sourceCatalog, err := systemCatalog()
	if err != nil {
		t.Fatal(err)
	}
	_, originalLock, err := renderFlake(sourceCatalog)
	if err != nil {
		t.Fatal(err)
	}
	if string(lock) != string(originalLock) || strings.Contains(string(lock), "runtimeSpec") {
		t.Fatal("guest flake lock differs from the embedded catalog and platform")
	}
	var pinned baseFlakeLock
	if err := json.Unmarshal(lock, &pinned); err != nil {
		t.Fatal(err)
	}
	declaration, err := os.ReadFile(filepath.Join(flake, "flake.nix"))
	if err != nil {
		t.Fatal(err)
	}
	release := pinned.Nodes["nixos-lima"].Original.Ref
	if release == "" || !strings.Contains(string(declaration), `"github:nixos-lima/nixos-lima/`+release+`"`) {
		t.Fatal("nixos-lima input differs from the locked image release")
	}
	clear(metadata)
	fresh, err := SystemModules()
	if err != nil || len(fresh) == 0 {
		t.Fatal("caller modified module metadata")
	}
}

func TestBundleSnapshotsWholeModuleTreeAndKeepsSecretsOutsideFlake(t *testing.T) {
	directory := t.TempDir()
	source := filepath.Join(directory, "module")
	if err := os.MkdirAll(filepath.Join(source, "nested"), 0o700); err != nil {
		t.Fatal(err)
	}
	for file, content := range map[string]string{"default.nix": "{...}: { imports = [ ./nested/feature.nix ]; }\n", "nested/feature.nix": "{...}: {}\n"} {
		if err := os.WriteFile(filepath.Join(source, file), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	cfg := config.Default()
	values := map[domain.EnvName]domain.EnvValue{"TOKEN": "test-secret-absent-from-flake", "LIMANIX_TEST_EMPTY": "", "LIMANIX_TEST_QUOTES": "single ' and double \"", "LIMANIX_TEST_LITERAL": "$(touch injected) `touch injected` $HOME", "LIMANIX_TEST_SPACES": "  a\tb  ", "LIMANIX_TEST_LINES": "line one\nline two\r\nend\n", "LIMANIX_TEST_BACKSLASH": "a\\b\\n\\\nlast\\", "LIMANIX_TEST_UNICODE": "Привет ✓"}
	cfg.Env = values
	runtimeDir := filepath.Join(directory, "runtime")
	flake, err := Prepare(cfg, runtimeDir, testGeneration, []modules.Source{{ID: "third-party:custom", Path: source}}, 501)
	if err != nil {
		t.Fatal(err)
	}
	assertBundledInterface(t, flake)
	assertOnlySharedCatalog(t, flake)
	if err := filepath.WalkDir(flake, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.Contains(string(data), "test-secret-absent-from-flake") {
			t.Fatalf("ENV secret copied to flake: %s", path)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(flake, "runtime.json"))
	if err != nil {
		t.Fatal(err)
	}
	var runtime struct {
		Modules []string `json:"modules"`
		User    struct {
			UID int `json:"uid"`
		} `json:"user"`
	}
	if err := json.Unmarshal(data, &runtime); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(runtime.Modules, append(publicDeclarationImports(t), "modules/0000/default.nix")) || runtime.User.UID != 501 {
		t.Fatalf("wrong runtime metadata: %s", data)
	}
	if err := os.WriteFile(filepath.Join(source, "nested", "feature.nix"), []byte("changed"), 0o600); err != nil {
		t.Fatal(err)
	}
	snapshot, err := os.ReadFile(filepath.Join(flake, "modules", "0000", "nested", "feature.nix"))
	if err != nil || string(snapshot) != "{...}: {}\n" {
		t.Fatal("generation module changed with registry source")
	}
	for _, file := range []string{"environment", "environment.sh"} {
		info, err := os.Stat(filepath.Join(runtimeDir, file))
		if err != nil || info.Mode().Perm() != 0o600 {
			t.Fatal("runtime ENV must be private on host")
		}
	}
	if _, err := Prepare(cfg, runtimeDir, testGeneration, nil, 501); err == nil {
		t.Fatal("existing bundle overwritten")
	}
	process := exec.Command("/bin/sh", "-c", `. "$1"; /usr/bin/env -0`, "sh", filepath.Join(runtimeDir, "environment.sh"))
	process.Dir = runtimeDir
	process.Env = []string{"PATH=/usr/bin:/bin"}
	output, err := process.Output()
	if err != nil {
		t.Fatal(err)
	}
	actual := map[string]string{}
	for _, item := range strings.Split(string(output), "\x00") {
		if item == "" {
			continue
		}
		name, value, _ := strings.Cut(item, "=")
		actual[name] = value
	}
	for name, value := range values {
		if actual[string(name)] != string(value) {
			t.Fatalf("ENV literal changed: %s", name)
		}
	}
	if _, err := os.Stat(filepath.Join(runtimeDir, "injected")); !os.IsNotExist(err) {
		t.Fatal("environment performed command substitution")
	}
}

func TestBundleWithoutSelectedModulesIncludesInterface(t *testing.T) {
	cfg := config.Default()
	cfg.NixOS.Modules = nil
	flake, err := Prepare(cfg, filepath.Join(t.TempDir(), "runtime"), testGeneration, nil, 501)
	if err != nil {
		t.Fatal(err)
	}
	assertBundledInterface(t, flake)
	assertOnlySharedCatalog(t, flake)
	data, err := os.ReadFile(filepath.Join(flake, "runtime.json"))
	if err != nil {
		t.Fatal(err)
	}
	var runtime runtimeConfig
	if err = json.Unmarshal(data, &runtime); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(runtime.Modules, publicDeclarationImports(t)) {
		t.Fatalf("empty selection must import only public declarations: %v", runtime.Modules)
	}
}

func assertBundledInterface(t *testing.T, flake string) {
	t.Helper()
	source, err := systemCatalog()
	if err != nil {
		t.Fatal(err)
	}
	for _, declaration := range source.PublicDeclarations() {
		if declaration == "_shared/test.nix" {
			t.Fatal("the shared test export must never be imported into a guest")
		}
		original, err := fs.ReadFile(source.Source(), declaration)
		if err != nil {
			t.Fatal(err)
		}
		copied, err := os.ReadFile(filepath.Join(flake, "modules", "lmx", filepath.FromSlash(declaration)))
		if err != nil || string(copied) != string(original) {
			t.Fatalf("public declaration %q missing or changed: %v", declaration, err)
		}
	}
	expected := source.Interface()
	actual, err := os.ReadFile(filepath.Join(flake, "interface.nix"))
	if err != nil || string(actual) != string(expected) {
		t.Fatalf("catalog interface missing or changed: %v", err)
	}
	declaration, err := os.ReadFile(filepath.Join(flake, "flake.nix"))
	if err != nil || !strings.Contains(string(declaration), "./interface.nix") {
		t.Fatalf("generated flake does not import the catalog interface: %v", err)
	}
}

func TestCatalogSnapshotRetainsUnselectedSiblingsAlongsideThirdPartyModules(t *testing.T) {
	source := t.TempDir()
	if err := os.WriteFile(filepath.Join(source, "default.nix"), []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	flake := t.TempDir()
	imports, err := copyModules(flake, []modules.Source{
		{ID: "third-party:custom", Path: source},
		{ID: "lmx:git"},
		{ID: "lmx:git"},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := append(publicDeclarationImports(t), "modules/0000/default.nix", "modules/lmx/git/default.nix", "modules/lmx/git/default.nix")
	if !reflect.DeepEqual(imports, want) {
		t.Fatalf("selection or shared module identity changed: got %v, want %v", imports, want)
	}
	if _, err := os.ReadFile(filepath.Join(flake, "modules", "lmx", "neovim", "default.nix")); err != nil {
		t.Fatalf("unselected sibling source is unavailable to Nix imports: %v", err)
	}
	entries, err := os.ReadDir(filepath.Join(flake, "modules"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected one catalog snapshot and one third-party snapshot, got %d", len(entries))
	}
}

func TestInvalidEnvironmentRejectedBeforeWriting(t *testing.T) {
	for _, values := range []map[domain.EnvName]domain.EnvValue{{"X;touch pwned": "x"}, {"X": "nul\x00value"}, {"X": "\ufeff"}} {
		cfg := config.Default()
		cfg.Env = values
		directory := filepath.Join(t.TempDir(), "absent")
		if _, err := Prepare(cfg, directory, testGeneration, nil, 501); err == nil {
			t.Fatal("invalid environment accepted")
		}
		if _, err := os.Stat(directory); !os.IsNotExist(err) {
			t.Fatal("invalid environment staged files before validation")
		}
	}
}

func publicDeclarationImports(t *testing.T) []string {
	t.Helper()
	source, err := systemCatalog()
	if err != nil {
		t.Fatal(err)
	}
	imports := make([]string, 0, len(source.PublicDeclarations()))
	for _, declaration := range source.PublicDeclarations() {
		imports = append(imports, "modules/lmx/"+declaration)
	}
	return imports
}

func assertOnlySharedCatalog(t *testing.T, flake string) {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(flake, "modules", "lmx"))
	if len(publicDeclarationImports(t)) == 0 {
		if !os.IsNotExist(err) {
			t.Fatalf("empty public interface copied optional catalog files: %v", err)
		}
		return
	}
	if err != nil || len(entries) != 1 || entries[0].Name() != "_shared" {
		t.Fatalf("selection without standard modules must copy only _shared: %v, %v", entries, err)
	}
}
