package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/limanix/client/internal/nixos/catalog"
)

func TestLocalSourceCommandPublishesCurrentCheckoutWithoutDownload(t *testing.T) {
	source, root := t.TempDir(), t.TempDir()
	lock, err := os.ReadFile("../../internal/nixos/catalog/testdata/flake.lock")
	if err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string][]byte{
		"LICENSE":                          []byte("fixture license\n"),
		"flake.lock":                       lock,
		"interface.nix":                    []byte("{ options = {}; }\n"),
		"platform.nix":                     []byte("{ services.lima.enable = true; }\n"),
		"catalog/modules/tool/default.nix": []byte("{}\n"),
		"catalog/modules/tool/module.toml": []byte("description = 'Tool'\n"),
	} {
		filename := filepath.Join(source, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(filename), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filename, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	directory := filepath.Join(root, "internal", "nixos", "resources")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	manifest := filepath.Join(t.TempDir(), "image.json")
	if err := os.WriteFile(manifest, []byte(imageManifest), 0o644); err != nil {
		t.Fatal(err)
	}
	var diagnostics bytes.Buffer
	if status := run(context.Background(), []string{"--source", source, "--version", "local-20261001", "--root", root}, &diagnostics); status != 1 || !strings.Contains(diagnostics.String(), "--image") {
		t.Fatalf("local source without a manifest accepted: status=%d, %s", status, diagnostics.String())
	}
	diagnostics.Reset()
	if status := run(context.Background(), []string{"--version", "v4", "--image", manifest, "--root", root}, &diagnostics); status != 2 || !strings.Contains(diagnostics.String(), "--image needs --source") {
		t.Fatalf("manifest accepted for a release tag: status=%d, %s", status, diagnostics.String())
	}
	diagnostics.Reset()
	args := []string{"--source", source, "--version", "local-20261001", "--image", manifest, "--root", root}
	if status := run(context.Background(), args, &diagnostics); status != 0 {
		t.Fatalf("local generation status=%d: %s", status, diagnostics.String())
	}
	if published, err := os.ReadFile(filepath.Join(directory, "image.json")); err != nil || string(published) != imageManifest {
		t.Fatalf("manifest not published: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(directory, "modules.zip"))
	if err != nil {
		t.Fatal(err)
	}
	bundled, err := catalog.Open(data)
	if err != nil || bundled.Version != "local-20261001" || bundled.Modules()["tool"].Description != "Tool" {
		t.Fatalf("local source not published: %v, %v", bundled, err)
	}
	diagnostics.Reset()
	if status := run(context.Background(), []string{"--source", source}, &diagnostics); status != 2 || !strings.Contains(diagnostics.String(), "--version is required") {
		t.Fatalf("missing build label accepted: status=%d, %s", status, diagnostics.String())
	}
	diagnostics.Reset()
	if status := run(context.Background(), []string{"--source", source, "--version", "../invalid", "--root", root}, &diagnostics); status != 1 {
		t.Fatalf("invalid label accepted: status=%d, %s", status, diagnostics.String())
	}
	retained, err := os.ReadFile(filepath.Join(directory, "modules.zip"))
	if err != nil || !bytes.Equal(data, retained) {
		t.Fatalf("invalid invocation replaced output: %v", err)
	}
}

const imageManifest = `{
  "version": "v4",
  "systems": {
    "aarch64-linux": {
      "url": "https://github.com/limanix/modules/releases/download/v4/limanix-aarch64.qcow2",
      "sha256": "10ecc1ab08c31c48f67106d9fdbb483340a52d963a945aa5c7a91905a1f68417"
    },
    "x86_64-linux": {
      "url": "https://github.com/limanix/modules/releases/download/v4/limanix-x86_64.qcow2",
      "sha256": "1bcc06b3c8f855c80ffd3df0ded38737728b5472494de98921fdb37d55613b24"
    }
  }
}
`
