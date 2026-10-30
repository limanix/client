package catalog

import (
	"errors"
	"io/fs"
	"reflect"
	"testing"
	"testing/fstest"
)

func TestVersionedSelections(t *testing.T) {
	files := fstest.MapFS{
		"modules/modules/tool/module.toml":       {Data: []byte("description = 'Tool'\ndefault = '26'\nversions = ['1.24', '26']\n")},
		"modules/modules/tool/default.nix":       {},
		"modules/modules/tool/versions/1.24.nix": {},
		"modules/modules/tool/versions/26.nix":   {},
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
		"tool":      "modules/tool/default.nix",
		"tool-1.24": "modules/tool/versions/1.24.nix",
		"tool-26":   "modules/tool/versions/26.nix",
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
		"modules/modules/console/module.toml":    {Data: []byte("description = 'Console'\n")},
		"modules/modules/console/default.nix":    {Data: []byte("{ imports = [ ../../shell/zsh/default.nix ../../_shared/internal/selection.nix ]; }\n")},
		"modules/shell/zsh/module.toml":          {Data: []byte("description = 'Zsh'\n")},
		"modules/shell/zsh/default.nix":          {Data: []byte("{ programs.zsh.enable = true; }\n")},
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
	if selected.EntryPoint != "modules/console/default.nix" {
		t.Fatalf("catalog entry point lost its directory: %s", selected.EntryPoint)
	}
	if _, err = fs.ReadFile(selected, "shell/zsh/default.nix"); err != nil {
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
			"modules/modules/tool/module.toml": {Data: []byte("description = 'Tool'\n")},
			"modules/modules/tool/default.nix": {},
			"modules/_shared":                  {},
		},
		"other reserved-looking directory": {
			"modules/modules/tool/module.toml": {Data: []byte("description = 'Tool'\n")},
			"modules/modules/tool/default.nix": {},
			"modules/_helpers/module.toml":     {Data: []byte("description = 'Helpers'\n")},
			"modules/_helpers/default.nix":     {},
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
		"modules/modules/tool/module.toml":     {Data: []byte("description = 'Tool'\ndefault = '26'\nversions = ['26']\n")},
		"modules/modules/tool/default.nix":     {},
		"modules/modules/tool/versions/26.nix": {},
		"modules/modules/tool-26/module.toml":  {Data: []byte("description = 'Another module'\n")},
		"modules/modules/tool-26/default.nix":  {},
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
		"modules/modules/tool/module.toml": {Data: []byte("description = 'Tool'\n")},
		"modules/modules/tool/default.nix": {},
		"modules/_shared/tool.nix":         {},
	}
	if _, err := readModules(files); err != nil {
		t.Fatalf("public area incorrectly reserves a module name: %v", err)
	}
}

func TestCatalogIgnoresRegularFilesBesideModules(t *testing.T) {
	files := fstest.MapFS{
		"modules/modules/tool/module.toml": {Data: []byte("description = 'Tool'\n")},
		"modules/modules/tool/default.nix": {},
		"modules/.DS_Store":                {Data: []byte("not a module")},
		"modules/README.md":                {Data: []byte("Catalog overview\n")},
	}
	index, err := readModules(files)
	if err != nil {
		t.Fatal(err)
	}
	if len(index) != 1 || index["tool"].entryPoint != "default.nix" {
		t.Fatalf("regular catalog files changed the module index: %v", index)
	}
}

func TestModulesLiveBelowTheirKind(t *testing.T) {
	files := fstest.MapFS{
		"modules/shell/zsh/module.toml":             {Data: []byte("description = 'Zsh'\n")},
		"modules/shell/zsh/default.nix":             {},
		"modules/toolchains/go/module.toml":         {Data: []byte("description = 'Go'\ndefault = '1.27'\nversions = ['1.26', '1.27']\n")},
		"modules/toolchains/go/default.nix":         {},
		"modules/toolchains/go/versions/1.26.nix":   {},
		"modules/toolchains/go/versions/1.27.nix":   {},
		"modules/modules/docker/module.toml":        {Data: []byte("description = 'Docker'\n")},
		"modules/modules/docker/default.nix":        {},
		"modules/_shared/languageSupport.nix":       {},
		"modules/README.md":                         {},
		"modules/modules/docker/README.md":          {},
		"modules/toolchains/go/versions/README.txt": {},
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
	for selector, want := range map[string]struct {
		entry      Entry
		entryPoint string
	}{
		"zsh":     {Entry{Kind: KindShell, Module: "zsh", Description: "Zsh"}, "shell/zsh/default.nix"},
		"go":      {Entry{Kind: KindToolchains, Module: "go", Description: "Go (default: 1.27)"}, "toolchains/go/default.nix"},
		"go-1.26": {Entry{Kind: KindToolchains, Module: "go", Line: "1.26", Description: "Go (version: 1.26)"}, "toolchains/go/versions/1.26.nix"},
		"docker":  {Entry{Kind: KindModules, Module: "docker", Description: "Docker"}, "modules/docker/default.nix"},
	} {
		if got := stored.Modules()[selector]; !reflect.DeepEqual(got, want.entry) {
			t.Fatalf("%s: entry %#v", selector, got)
		}
		selected, err := stored.Module(selector)
		if err != nil || selected.EntryPoint != want.entryPoint {
			t.Fatalf("%s: entry point %q, %v", selector, selected.EntryPoint, err)
		}
	}
	if len(stored.Modules()) != 5 {
		t.Fatalf("selectors: %v", stored.Modules())
	}
}

func TestModuleOutsideAKindOrTwiceIsRejected(t *testing.T) {
	for name, files := range map[string]fstest.MapFS{
		"flat module": {
			"modules/tool/module.toml": {Data: []byte("description = 'Tool'\n")},
			"modules/tool/default.nix": {},
		},
		"unknown kind": {
			"modules/tools/tool/module.toml": {Data: []byte("description = 'Tool'\n")},
			"modules/tools/tool/default.nix": {},
		},
		"same name in two kinds": {
			"modules/shell/tool/module.toml":   {Data: []byte("description = 'Tool'\n")},
			"modules/shell/tool/default.nix":   {},
			"modules/modules/tool/module.toml": {Data: []byte("description = 'Tool'\n")},
			"modules/modules/tool/default.nix": {},
		},
		"reserved name in a kind": {
			"modules/modules/pins/module.toml": {Data: []byte("description = 'Pins'\n")},
			"modules/modules/pins/default.nix": {},
		},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := readModules(files); !errors.Is(err, ErrMetadata) {
				t.Fatalf("invalid layout accepted: %v", err)
			}
		})
	}
}
