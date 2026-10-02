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
		"LICENSE":                  []byte("fixture license\n"),
		"flake.lock":               lock,
		"interface.nix":            []byte("{ options = {}; }\n"),
		"catalog/tool/default.nix": []byte("{}\n"),
		"catalog/tool/module.toml": []byte("description = 'Tool'\n"),
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
	args := []string{"--source", source, "--version", "local-20261001", "--root", root}
	var diagnostics bytes.Buffer
	if status := run(context.Background(), args, &diagnostics); status != 0 {
		t.Fatalf("local generation status=%d: %s", status, diagnostics.String())
	}
	data, err := os.ReadFile(filepath.Join(directory, "modules.zip"))
	if err != nil {
		t.Fatal(err)
	}
	bundled, err := catalog.Open(data)
	if err != nil || bundled.Version != "local-20261001" || bundled.Modules()["tool"] != "Tool" {
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
