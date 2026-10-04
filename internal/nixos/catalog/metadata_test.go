package catalog

import (
	"errors"
	"io/fs"
	"testing"
	"testing/fstest"
)

func TestVersionedSelections(t *testing.T) {
	files := fstest.MapFS{
		"modules/tool/module.toml":       {Data: []byte("description = 'Tool'\ndefault = '26'\nversions = ['1.24', '26']\n")},
		"modules/tool/default.nix":       {},
		"modules/tool/versions/1.24.nix": {},
		"modules/tool/versions/26.nix":   {},
	}
	index, err := readModules(files)
	if err != nil {
		t.Fatal(err)
	}
	source, err := fs.Sub(files, "modules")
	if err != nil {
		t.Fatal(err)
	}
	catalog := &Catalog{files: source, modules: index}
	for selector, entry := range map[string]string{
		"tool":      "tool/default.nix",
		"tool-1.24": "tool/versions/1.24.nix",
		"tool-26":   "tool/versions/26.nix",
	} {
		selected, err := catalog.Module(selector)
		if err != nil || selected.EntryPoint != entry {
			t.Fatalf("%s: entry %q, error %v", selector, selected.EntryPoint, err)
		}
	}
	if _, err := catalog.Module("tool-1.99"); !errors.Is(err, ErrModule) {
		t.Fatalf("unknown version accepted: %v", err)
	}
}

func TestSelectedModuleRetainsSiblingImports(t *testing.T) {
	files := fstest.MapFS{
		"modules/console/module.toml":            {Data: []byte("description = 'Console'\n")},
		"modules/console/default.nix":            {Data: []byte("{ imports = [ ../shell/default.nix ../_shared/internal/selection.nix ]; }\n")},
		"modules/shell/module.toml":              {Data: []byte("description = 'Shell'\n")},
		"modules/shell/default.nix":              {Data: []byte("{ programs.zsh.enable = true; }\n")},
		"modules/_shared/internal/selection.nix": {Data: []byte("{ options = {}; }\n")},
	}
	index, err := readModules(files)
	if err != nil {
		t.Fatal(err)
	}
	source, err := fs.Sub(files, "modules")
	if err != nil {
		t.Fatal(err)
	}
	stored := &Catalog{files: source, modules: index}
	selected, err := stored.Module("console")
	if err != nil {
		t.Fatal(err)
	}
	if selected.EntryPoint != "console/default.nix" {
		t.Fatalf("catalog entry point lost its directory: %s", selected.EntryPoint)
	}
	if _, err = fs.ReadFile(selected, "shell/default.nix"); err != nil {
		t.Fatalf("selected module cannot import its sibling: %v", err)
	}
	if _, err = fs.ReadFile(selected, "_shared/internal/selection.nix"); err != nil {
		t.Fatalf("selected module cannot import shared declarations: %v", err)
	}
	if _, exists := stored.Modules()["_shared"]; exists {
		t.Fatal("shared declarations were listed as a module")
	}
	if _, err = stored.Module("_shared"); !errors.Is(err, ErrModule) {
		t.Fatalf("shared declarations accepted as a selector: %v", err)
	}
}

func TestInvalidCatalogDirectories(t *testing.T) {
	for name, files := range map[string]fstest.MapFS{
		"shared is a file": {
			"modules/tool/module.toml": {Data: []byte("description = 'Tool'\n")},
			"modules/tool/default.nix": {},
			"modules/_shared":          {},
		},
		"other reserved-looking directory": {
			"modules/tool/module.toml":     {Data: []byte("description = 'Tool'\n")},
			"modules/tool/default.nix":     {},
			"modules/_helpers/module.toml": {Data: []byte("description = 'Helpers'\n")},
			"modules/_helpers/default.nix": {},
		},
		"only shared declarations": {
			"modules/_shared/internal/selection.nix": {},
		},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := readModules(files); !errors.Is(err, ErrMetadata) {
				t.Fatalf("invalid catalog accepted: %v", err)
			}
		})
	}
}

func TestAmbiguousModuleNameRejected(t *testing.T) {
	files := fstest.MapFS{
		"modules/tool/module.toml":     {Data: []byte("description = 'Tool'\ndefault = '26'\nversions = ['26']\n")},
		"modules/tool/default.nix":     {},
		"modules/tool/versions/26.nix": {},
		"modules/tool-26/module.toml":  {Data: []byte("description = 'Another module'\n")},
		"modules/tool-26/default.nix":  {},
	}
	if _, err := readModules(files); !errors.Is(err, ErrMetadata) {
		t.Fatalf("module name indistinguishable from a version selector accepted: %v", err)
	}
}

func TestInvalidVersionMetadata(t *testing.T) {
	for name, declaration := range map[string]string{
		"missing default":                   "versions = ['1.24']",
		"unknown default":                   "default = '1.27'\nversions = ['1.24']",
		"default without versions":          "default = '1.24'",
		"empty default without versions":    "default = ''",
		"empty default with empty versions": "default = ''\nversions = []",
		"duplicate":                         "default = '1.24'\nversions = ['1.24', '1.24']",
		"unsafe":                            "default = '../escape'\nversions = ['../escape']",
		"missing entry":                     "default = '1.24'\nversions = ['1.24']",
		"unknown field":                     "defaults = '1.24'",
	} {
		t.Run(name, func(t *testing.T) {
			files := fstest.MapFS{
				"module.toml": {Data: []byte("description = 'Tool'\n" + declaration)},
				"default.nix": {},
			}
			if _, err := readMetadata(files, "."); !errors.Is(err, ErrMetadata) {
				t.Fatalf("invalid metadata accepted: %v", err)
			}
		})
	}
}

func TestReservedCatalogModuleNames(t *testing.T) {
	for _, name := range []string{"capabilities", "internal", "pins"} {
		t.Run(name, func(t *testing.T) {
			files := fstest.MapFS{
				"modules/" + name + "/module.toml": {Data: []byte("description = 'Reserved'\n")},
				"modules/" + name + "/default.nix": {},
			}
			if _, err := readModules(files); !errors.Is(err, ErrMetadata) {
				t.Fatalf("reserved module name accepted: %v", err)
			}
		})
	}
	files := fstest.MapFS{
		"modules/tool/module.toml": {Data: []byte("description = 'Tool'\n")},
		"modules/tool/default.nix": {},
		"modules/_shared/tool.nix": {},
	}
	if _, err := readModules(files); err != nil {
		t.Fatalf("public area incorrectly reserves a module name: %v", err)
	}
}

func TestCatalogIgnoresRegularFilesBesideModules(t *testing.T) {
	files := fstest.MapFS{
		"modules/tool/module.toml": {Data: []byte("description = 'Tool'\n")},
		"modules/tool/default.nix": {},
		"modules/.DS_Store":        {Data: []byte("not a module")},
		"modules/README.md":        {Data: []byte("Catalog overview\n")},
	}
	index, err := readModules(files)
	if err != nil {
		t.Fatal(err)
	}
	if len(index) != 1 || index["tool"].entryPoint != "default.nix" {
		t.Fatalf("regular catalog files changed the module index: %v", index)
	}
}
