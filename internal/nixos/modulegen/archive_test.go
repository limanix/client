package modulegen

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/limanix/client/internal/nixos/catalog"
)

func TestPackRetainsCatalogPinAndMapsSourceDirectory(t *testing.T) {
	files := catalogFiles(t)
	files["flake.nix"] = []byte("source declaration\n")
	files["README.md"] = []byte("repository documentation\n")
	files["guides/index.md"] = []byte("catalog documentation\n")
	files["modules/ignored/default.nix"] = []byte("{}\n")
	data := packFixture(t, files)
	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	lock, err := fs.ReadFile(archive, "flake.lock")
	if err != nil || !bytes.Equal(lock, files["flake.lock"]) {
		t.Fatalf("catalog pin was not preserved: %v", err)
	}
	module, err := fs.ReadFile(archive, "modules/tool/default.nix")
	if err != nil || !bytes.Equal(module, files["catalog/tool/default.nix"]) {
		t.Fatalf("catalog module was not mapped into the embedded module tree: %v", err)
	}
	for _, name := range []string{"flake.nix", "README.md", "guides/index.md", "catalog/tool/default.nix", "modules/ignored/default.nix"} {
		if _, err := fs.Stat(archive, name); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("unexpected archive entry %s: %v", name, err)
		}
	}
	if _, err := catalog.Open(data); err != nil {
		t.Fatalf("packed catalog is invalid: %v", err)
	}
}

func TestCheckExistingRequiresCatalogPin(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "modules.zip")
	files := catalogFiles(t)
	if err := os.WriteFile(filename, packFixture(t, files), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := checkExisting(filename, "v4"); err != nil {
		t.Fatalf("valid existing catalog rejected: %v", err)
	}
	if err := checkExisting(filename, "v5"); err == nil {
		t.Fatal("wrong existing catalog version accepted")
	}
	delete(files, "flake.lock")
	if err := os.WriteFile(filename, packFixture(t, files), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := checkExisting(filename, "v4"); !errors.Is(err, catalog.ErrArchive) {
		t.Fatalf("old same-tag archive without the NixOS pin must be invalidated: %v", err)
	}
}

func catalogFiles(t *testing.T) map[string][]byte {
	t.Helper()
	lock, err := os.ReadFile("../catalog/testdata/flake.lock")
	if err != nil {
		t.Fatal(err)
	}
	return map[string][]byte{
		"flake.lock":               lock,
		"LICENSE":                  []byte("test license\n"),
		"catalog/tool/module.toml": []byte("description = 'Tool'\n"),
		"catalog/tool/default.nix": []byte("{}\n"),
	}
}

func packFixture(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var source bytes.Buffer
	compressed := gzip.NewWriter(&source)
	writer := tar.NewWriter(compressed)
	for name, data := range files {
		header := &tar.Header{Name: "modules-v4/" + name, Mode: 0o644, Size: int64(len(data)), Typeflag: tar.TypeReg}
		if err := writer.WriteHeader(header); err != nil {
			t.Fatal(err)
		}
		if _, err := writer.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := compressed.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := pack(context.Background(), &source, "v4")
	if err != nil {
		t.Fatal(err)
	}
	return data
}
