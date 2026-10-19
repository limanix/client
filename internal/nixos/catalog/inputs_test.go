package catalog

import (
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"
)

func TestCatalogNixosLimaFollowsTheNixpkgsPin(t *testing.T) {
	source, err := Open(catalogArchive(t, nixpkgsFixture(t)))
	if err != nil {
		t.Fatal(err)
	}
	input, nodes := source.NixosLima()
	if input.URL != "github:nixos-lima/nixos-lima/v0.2.1" || input.Node != "nixos-lima" {
		t.Fatalf("nixos-lima input: %+v", input)
	}
	var names []string
	for name := range nodes {
		names = append(names, name)
	}
	slices.Sort(names)
	if want := []string{"flake-utils", "nixlib", "nixos-generators", "nixos-lima", "systems"}; !slices.Equal(names, want) {
		t.Fatalf("nixos-lima graph: got %v, want %v", names, want)
	}
	var node struct {
		Inputs map[string]json.RawMessage `json:"inputs"`
	}
	var follows []string
	if err := json.Unmarshal(nodes["nixos-lima"], &node); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(node.Inputs["nixpkgs"], &follows); err != nil || !slices.Equal(follows, []string{"nixpkgs"}) {
		t.Fatalf("nixos-lima must follow nixpkgs: %s, %v", node.Inputs["nixpkgs"], err)
	}
	nodes["nixos-lima"] = nil
	if _, again := source.NixosLima(); len(again["nixos-lima"]) == 0 {
		t.Fatal("returned graph is not an independent copy")
	}
}

func TestCatalogPlatform(t *testing.T) {
	source, err := Open(catalogArchive(t, nixpkgsFixture(t)))
	if err != nil {
		t.Fatal(err)
	}
	platform := source.Platform()
	if string(platform) != "{ services.lima.enable = true; }\n" {
		t.Fatalf("platform module: %q", platform)
	}
	platform[0] = '#'
	if string(source.Platform()) != "{ services.lima.enable = true; }\n" {
		t.Fatal("returned platform module is not an independent copy")
	}
}

func TestCatalogRejectsMissingNonRegularOrEmptyPlatform(t *testing.T) {
	for name, files := range map[string]map[string][]byte{
		"missing":   {},
		"directory": {"platform.nix/": nil},
		"empty":     {"platform.nix": {}},
	} {
		t.Run(name, func(t *testing.T) {
			archive := map[string][]byte{
				"version":                  []byte("v4\n"),
				"LICENSE":                  []byte("test license\n"),
				"flake.lock":               nixpkgsFixture(t),
				"interface.nix":            []byte("{ options = {}; }\n"),
				"modules/tool/module.toml": []byte("description = 'Tool'\n"),
				"modules/tool/default.nix": []byte("{}\n"),
			}
			for path, data := range files {
				archive[path] = data
			}
			if _, err := Open(catalogArchiveFiles(t, archive)); !errors.Is(err, ErrArchive) || !strings.Contains(err.Error(), "platform.nix") {
				t.Fatalf("catalog without a usable platform.nix accepted: %v", err)
			}
		})
	}
}

func TestCatalogRejectsInvalidNixosLima(t *testing.T) {
	tests := []struct {
		name  string
		path  []string
		value any
	}{
		{"missing root input", []string{"nodes", "root", "inputs", "nixos-lima"}, nil},
		{"follows root input", []string{"nodes", "root", "inputs", "nixos-lima"}, []string{"other"}},
		{"missing node", []string{"nodes", "root", "inputs", "nixos-lima"}, "missing"},
		{"not following nixpkgs", []string{"nodes", "nixos-lima", "inputs", "nixpkgs"}, []string{"other"}},
		{"own nixpkgs node", []string{"nodes", "nixos-lima", "inputs", "nixpkgs"}, "nixpkgs"},
		{"missing dependency", []string{"nodes", "nixos-lima", "inputs", "flake-utils"}, "missing"},
		{"dependency names root", []string{"nodes", "nixos-lima", "inputs", "flake-utils"}, "root"},
		{"dependency follows unknown input", []string{"nodes", "nixos-generators", "inputs", "nixpkgs"}, []string{"other", "nixpkgs"}},
		{"unknown node field", []string{"nodes", "nixos-lima", "dir"}, "subdir"},
		{"not a flake", []string{"nodes", "nixos-lima", "flake"}, false},
		{"locked owner", []string{"nodes", "nixos-lima", "locked", "owner"}, "other"},
		{"original repo", []string{"nodes", "nixos-lima", "original", "repo"}, "other"},
		{"empty ref", []string{"nodes", "nixos-lima", "original", "ref"}, ""},
		{"short rev", []string{"nodes", "nixos-lima", "locked", "rev"}, "24916c8"},
		{"invalid hash", []string{"nodes", "nixos-lima", "locked", "narHash"}, "sha256-invalid"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var lock map[string]any
			if err := json.Unmarshal(nixpkgsFixture(t), &lock); err != nil {
				t.Fatal(err)
			}
			field := lock
			for _, key := range test.path[:len(test.path)-1] {
				field = field[key].(map[string]any)
			}
			field[test.path[len(test.path)-1]] = test.value
			data, err := json.Marshal(lock)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := Open(catalogArchive(t, data)); !errors.Is(err, ErrArchive) || !strings.Contains(err.Error(), "flake.lock") {
				t.Fatalf("invalid nixos-lima pin must report a flake.lock archive error: %v", err)
			}
		})
	}
}
