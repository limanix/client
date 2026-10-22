package nixos

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/limanix/client/internal/nixos/catalog"
)

func TestCatalogPinsControlGuestFlakeAndLock(t *testing.T) {
	for _, ref := range []string{"nixos-26.05", "nixos-unstable"} {
		t.Run(ref, func(t *testing.T) {
			pin := `{"locked":{"type":"github","owner":"NixOS","repo":"nixpkgs","rev":"1111111111111111111111111111111111111111","narHash":"sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="},"original":{"type":"github","owner":"NixOS","repo":"nixpkgs","ref":"` + ref + `"}}`
			source := catalogWithPin(t, pin)
			declaration, data, err := renderFlake(source)
			if err != nil {
				t.Fatal(err)
			}
			for _, want := range []string{
				`nixpkgs.url = "github:NixOS/nixpkgs/` + ref + `";`,
				`url = "github:nixos-lima/nixos-lima/v0.2.1";`,
				"./platform.nix", "./defaults.nix",
			} {
				if !strings.Contains(string(declaration), want) {
					t.Fatalf("flake lacks %q: %s", want, declaration)
				}
			}
			var actual struct {
				Nodes   map[string]json.RawMessage `json:"nodes"`
				Root    string                     `json:"root"`
				Version int                        `json:"version"`
			}
			if err := json.Unmarshal(data, &actual); err != nil {
				t.Fatal(err)
			}
			if actual.Root != "root" || actual.Version != 7 {
				t.Fatalf("lock header: %+v", actual)
			}
			assertSameJSON(t, actual.Nodes["nixpkgs"], []byte(pin))
			assertSameJSON(t, actual.Nodes["root"], []byte(`{"inputs":{"nixpkgs":"nixpkgs","nixos-lima":"nixos-lima"}}`))
			_, graph := source.NixosLima()
			if len(actual.Nodes) != len(graph)+2 {
				t.Fatalf("lock nodes: got %d, want the nixos-lima graph of %d plus nixpkgs and root", len(actual.Nodes), len(graph))
			}
			for name, node := range graph {
				assertSameJSON(t, actual.Nodes[name], node)
			}
		})
	}
}

// catalogWithPin builds a catalog whose nixpkgs node is renamed, as Nix does on conflicts, to keep the guest lock independent of node names.
func catalogWithPin(t *testing.T, pin string) *catalog.Catalog {
	t.Helper()
	lock, err := os.ReadFile("catalog/testdata/flake.lock")
	if err != nil {
		t.Fatal(err)
	}
	var nodes struct {
		Nodes map[string]json.RawMessage `json:"nodes"`
	}
	if err := json.Unmarshal(lock, &nodes); err != nil {
		t.Fatal(err)
	}
	delete(nodes.Nodes, "nixpkgs")
	nodes.Nodes["packages"] = json.RawMessage(pin)
	nodes.Nodes["root"] = json.RawMessage(`{"inputs":{"nixpkgs":"packages","nixos-lima":"nixos-lima"}}`)
	lock, err = json.Marshal(map[string]any{"version": 7, "root": "root", "nodes": nodes.Nodes})
	if err != nil {
		t.Fatal(err)
	}
	var buffer bytes.Buffer
	archive := zip.NewWriter(&buffer)
	if err := archive.SetComment(catalog.Repository); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"version":                          "test-pin",
		"LICENSE":                          "test fixture",
		"interface.nix":                    "{}",
		"platform.nix":                     "{ services.lima.enable = true; }",
		"modules/modules/test/default.nix": "{}",
		"modules/modules/test/module.toml": "description = 'Test'",
		"flake.lock":                       string(lock),
	}
	for name, data := range files {
		file, err := archive.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := file.Write([]byte(data)); err != nil {
			t.Fatal(err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	source, err := catalog.Open(buffer.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	return source
}

func assertSameJSON(t *testing.T, actual, expected []byte) {
	t.Helper()
	var left, right any
	if err := json.Unmarshal(actual, &left); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(expected, &right); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(left, right) {
		t.Fatalf("JSON differs: got %s, want %s", actual, expected)
	}
}
