package nixos

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/limanix/client/internal/nixos/catalog"
)

func TestCatalogPinControlsGuestFlakeAndPreservesPlatform(t *testing.T) {
	platform, err := resources.ReadFile("resources/flake.lock.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	var expected struct {
		Nodes map[string]json.RawMessage `json:"nodes"`
	}
	if err := json.Unmarshal(platform, &expected); err != nil {
		t.Fatal(err)
	}
	if _, exists := expected.Nodes["nixpkgs"]; exists {
		t.Fatal("client still owns a nixpkgs pin")
	}
	for _, ref := range []string{"nixos-26.05", "nixos-unstable"} {
		t.Run(ref, func(t *testing.T) {
			pin := `{"locked":{"type":"github","owner":"NixOS","repo":"nixpkgs","rev":"1111111111111111111111111111111111111111","narHash":"sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="},"original":{"type":"github","owner":"NixOS","repo":"nixpkgs","ref":"` + ref + `"}}`
			source := catalogWithPin(t, pin)
			declaration, data, err := renderFlake(source)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(declaration), `nixpkgs.url = "github:NixOS/nixpkgs/`+ref+`";`) {
				t.Fatalf("catalog ref not used in flake: %s", declaration)
			}
			var actual struct {
				Nodes map[string]json.RawMessage `json:"nodes"`
			}
			if err := json.Unmarshal(data, &actual); err != nil {
				t.Fatal(err)
			}
			assertSameJSON(t, actual.Nodes["nixpkgs"], []byte(pin))
			if len(actual.Nodes) != len(expected.Nodes)+1 {
				t.Fatal("unexpected changes to the platform dependency graph")
			}
			for name, original := range expected.Nodes {
				assertSameJSON(t, actual.Nodes[name], original)
			}
		})
	}
}

func catalogWithPin(t *testing.T, pin string) *catalog.Catalog {
	t.Helper()
	var buffer bytes.Buffer
	archive := zip.NewWriter(&buffer)
	if err := archive.SetComment(catalog.Repository); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"version":                  "test-pin",
		"LICENSE":                  "test fixture",
		"interface.nix":            "{}",
		"modules/test/default.nix": "{}",
		"modules/test/module.toml": "description = 'Test'",
		"flake.lock":               `{"version":7,"root":"root","nodes":{"root":{"inputs":{"nixpkgs":"packages"}},"packages":` + pin + `}}`,
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
