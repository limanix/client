package catalog

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
)

func TestCatalogNixpkgs(t *testing.T) {
	data := nixpkgsFixture(t)
	var lock struct {
		Nodes map[string]json.RawMessage `json:"nodes"`
	}
	if err := json.Unmarshal(data, &lock); err != nil {
		t.Fatal(err)
	}
	catalog, err := Open(catalogArchive(t, data))
	if err != nil {
		t.Fatal(err)
	}
	url, node := catalog.Nixpkgs()
	if url != "github:NixOS/nixpkgs/nixos-26.05" || !bytes.Equal(node, lock.Nodes["nixpkgs"]) {
		t.Fatalf("unexpected pin: %s\n%s", url, node)
	}
	node[0] = '!'
	_, again := catalog.Nixpkgs()
	if !bytes.Equal(again, lock.Nodes["nixpkgs"]) {
		t.Fatal("caller changed the catalog pin")
	}
}

func TestCatalogInterface(t *testing.T) {
	archiveData := catalogArchive(t, nixpkgsFixture(t))
	catalog, err := Open(archiveData)
	if err != nil {
		t.Fatal(err)
	}
	clear(archiveData)
	data := catalog.Interface()
	if string(data) != "{ options = {}; }\n" {
		t.Fatalf("public interface changed: %q", data)
	}
	data[0] = '!'
	again := catalog.Interface()
	if string(again) != "{ options = {}; }\n" {
		t.Fatalf("caller changed the public interface: %q", again)
	}
}

func TestCatalogRejectsMissingNonRegularOrEmptyInterface(t *testing.T) {
	for _, name := range []string{"", "interface.nix/", "interface.nix"} {
		data := catalogArchiveWithInterface(t, nixpkgsFixture(t), name, nil)
		if _, err := Open(data); !errors.Is(err, ErrArchive) || !strings.Contains(err.Error(), "interface.nix") {
			t.Fatalf("interface %q must be rejected as an archive error: %v", name, err)
		}
	}
}

func TestCatalogRejectsDamagedInterface(t *testing.T) {
	data := catalogArchive(t, nixpkgsFixture(t))
	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range archive.File {
		if entry.Name != "interface.nix" {
			continue
		}
		offset, err := entry.DataOffset()
		if err != nil {
			t.Fatal(err)
		}
		data[offset] ^= 0xff
		if _, err := Open(data); !errors.Is(err, ErrArchive) || !strings.Contains(err.Error(), "interface.nix") {
			t.Fatalf("damaged interface must be rejected as an archive error: %v", err)
		}
		return
	}
	t.Fatal("fixture does not contain interface.nix")
}

func TestCatalogNixpkgsAllowsRenamedNodesAndUnstable(t *testing.T) {
	var lock map[string]any
	if err := json.Unmarshal(nixpkgsFixture(t), &lock); err != nil {
		t.Fatal(err)
	}
	nodes := lock["nodes"].(map[string]any)
	node := nodes["nixpkgs"].(map[string]any)
	node["original"].(map[string]any)["ref"] = "nixos-unstable"
	nodes["pinned"] = node
	delete(nodes, "nixpkgs")
	nodes["catalog"] = map[string]any{"inputs": map[string]any{"nixpkgs": "pinned"}}
	delete(nodes, "root")
	lock["root"] = "catalog"
	data, err := json.Marshal(lock)
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := Open(catalogArchive(t, data))
	if err != nil {
		t.Fatal(err)
	}
	url, _ := catalog.Nixpkgs()
	if url != "github:NixOS/nixpkgs/nixos-unstable" {
		t.Fatalf("unexpected pin URL: %q", url)
	}
}

func TestCatalogRejectsInvalidNixpkgs(t *testing.T) {
	tests := []struct {
		name  string
		path  []string
		value any
	}{
		{"lock version", []string{"version"}, 6},
		{"missing root", []string{"root"}, "missing"},
		{"missing root input", []string{"nodes", "root", "inputs", "nixpkgs"}, nil},
		{"follows input", []string{"nodes", "root", "inputs", "nixpkgs"}, []string{"other", "nixpkgs"}},
		{"missing node", []string{"nodes", "root", "inputs", "nixpkgs"}, "missing"},
		{"input dependency", []string{"nodes", "nixpkgs", "inputs"}, map[string]string{"other": "missing"}},
		{"null inputs", []string{"nodes", "nixpkgs", "inputs"}, nil},
		{"not a flake", []string{"nodes", "nixpkgs", "flake"}, false},
		{"null flake", []string{"nodes", "nixpkgs", "flake"}, nil},
		{"unknown node field", []string{"nodes", "nixpkgs", "dir"}, "subdir"},
		{"incorrect attribute case", []string{"nodes", "nixpkgs", "locked", "Type"}, "github"},
		{"original type", []string{"nodes", "nixpkgs", "original", "type"}, "git"},
		{"original owner", []string{"nodes", "nixpkgs", "original", "owner"}, "other"},
		{"original repo", []string{"nodes", "nixpkgs", "original", "repo"}, "other"},
		{"original rev", []string{"nodes", "nixpkgs", "original", "rev"}, strings.Repeat("b", 40)},
		{"locked type", []string{"nodes", "nixpkgs", "locked", "type"}, "git"},
		{"locked owner", []string{"nodes", "nixpkgs", "locked", "owner"}, "other"},
		{"locked repo", []string{"nodes", "nixpkgs", "locked", "repo"}, "other"},
		{"locked directory", []string{"nodes", "nixpkgs", "locked", "dir"}, "subdir"},
		{"locked host", []string{"nodes", "nixpkgs", "locked", "host"}, "example.com"},
		{"null modification time", []string{"nodes", "nixpkgs", "locked", "lastModified"}, nil},
		{"empty ref", []string{"nodes", "nixpkgs", "original", "ref"}, ""},
		{"interpolation ref", []string{"nodes", "nixpkgs", "original", "ref"}, "${builtins.abort \"unsafe\"}"},
		{"quoted ref", []string{"nodes", "nixpkgs", "original", "ref"}, "nixos-26.05\""},
		{"query ref", []string{"nodes", "nixpkgs", "original", "ref"}, "nixos-26.05?dir=other"},
		{"short rev", []string{"nodes", "nixpkgs", "locked", "rev"}, "a311611"},
		{"uppercase rev", []string{"nodes", "nixpkgs", "locked", "rev"}, strings.Repeat("A", 40)},
		{"wrong hash algorithm", []string{"nodes", "nixpkgs", "locked", "narHash"}, "sha512-Z+vUNbfd2FIKkWOTkcT7RYlh3oFCnig/d2eXD1SWf2E="},
		{"invalid hash encoding", []string{"nodes", "nixpkgs", "locked", "narHash"}, "sha256-invalid"},
		{"null hash", []string{"nodes", "nixpkgs", "locked", "narHash"}, nil},
		{"newline hash", []string{"nodes", "nixpkgs", "locked", "narHash"}, "sha256-Z+vUNbfd2FIKkWOTkcT7RYlh3oFCnig/d2eXD1SWf2E=\n"},
		{"short hash", []string{"nodes", "nixpkgs", "locked", "narHash"}, "sha256-YQ=="},
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
				t.Fatalf("invalid pin must report a flake.lock archive error: %v", err)
			}
		})
	}
	for name, data := range map[string][]byte{
		"missing lock": nil,
		"invalid JSON": []byte("{"),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Open(catalogArchive(t, data)); !errors.Is(err, ErrArchive) || !strings.Contains(err.Error(), "flake.lock") {
				t.Fatalf("invalid pin must report a flake.lock archive error: %v", err)
			}
		})
	}
}

func nixpkgsFixture(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/flake.lock")
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func catalogArchive(t *testing.T, lock []byte) []byte {
	t.Helper()
	return catalogArchiveWithInterface(t, lock, "interface.nix", []byte("{ options = {}; }\n"))
}

func catalogArchiveWithInterface(t *testing.T, lock []byte, interfacePath string, interfaceData []byte) []byte {
	t.Helper()
	files := map[string][]byte{
		"version":                  []byte("v4\n"),
		"LICENSE":                  []byte("test license\n"),
		"modules/tool/module.toml": []byte("description = 'Tool'\n"),
		"modules/tool/default.nix": []byte("{}\n"),
	}
	if lock != nil {
		files["flake.lock"] = lock
	}
	if interfacePath != "" {
		files[interfacePath] = interfaceData
	}
	return catalogArchiveFiles(t, files)
}

func catalogArchiveFiles(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	if err := writer.SetComment(Repository); err != nil {
		t.Fatal(err)
	}
	for name, data := range files {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasSuffix(name, "/") {
			if _, err = entry.Write(data); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}
