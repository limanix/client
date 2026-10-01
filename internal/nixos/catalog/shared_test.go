package catalog

import (
	"errors"
	"io/fs"
	"slices"
	"testing"
)

func TestPublicDeclarationsAvailableWithoutModuleSelection(t *testing.T) {
	files := sharedFixture(t)
	files["modules/_shared/zulu.nix"] = []byte("{ options = {}; }\n")
	files["modules/_shared/README.md"] = []byte("Shared declarations\n")
	catalog, err := Open(catalogArchiveFiles(t, files))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"_shared/editor.nix", "_shared/zulu.nix"}
	got := catalog.PublicDeclarations()
	if !slices.Equal(got, want) {
		t.Fatalf("public declarations: got %v, want %v", got, want)
	}
	got[0] = "_shared/internal/selection.nix"
	if !slices.Equal(catalog.PublicDeclarations(), want) {
		t.Fatal("caller changed the catalog declaration list")
	}
	for _, name := range append(want, "_shared/internal/selection.nix", "tool/default.nix") {
		if _, err = fs.ReadFile(catalog.Source(), name); err != nil {
			t.Fatalf("source %s is unavailable without selecting a module: %v", name, err)
		}
	}
	if _, err = fs.Stat(catalog.Source(), "interface.nix"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("source must be rooted at catalog module directories: %v", err)
	}
	for _, name := range []string{"_shared", "editor", "zulu", "internal"} {
		if _, ok := catalog.Modules()[name]; ok {
			t.Fatalf("shared declaration became a selectable module: %s", name)
		}
	}
}

func TestCatalogRejectsInvalidPublicDeclarations(t *testing.T) {
	for _, name := range []string{"modules/_shared/module.toml", "modules/_shared/invalid.nix/"} {
		t.Run(name, func(t *testing.T) {
			files := sharedFixture(t)
			files[name] = []byte{}
			if _, err := Open(catalogArchiveFiles(t, files)); !errors.Is(err, ErrMetadata) {
				t.Fatalf("invalid public declaration accepted: %v", err)
			}
		})
	}
}

func sharedFixture(t *testing.T) map[string][]byte {
	t.Helper()
	return map[string][]byte{
		"version":                                []byte("v4\n"),
		"LICENSE":                                []byte("test license\n"),
		"flake.lock":                             nixpkgsFixture(t),
		"interface.nix":                          []byte("{ options = {}; }\n"),
		"modules/tool/module.toml":               []byte("description = 'Tool'\n"),
		"modules/tool/default.nix":               []byte("{}\n"),
		"modules/_shared/editor.nix":             []byte("{ options = {}; }\n"),
		"modules/_shared/internal/selection.nix": []byte("{ options = {}; }\n"),
	}
}
