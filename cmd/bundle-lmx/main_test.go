package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func checkout(t *testing.T) string {
	t.Helper()
	source := t.TempDir()
	write := func(name string, data []byte) {
		t.Helper()
		filename := filepath.Join(source, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(filename), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filename, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("Cargo.toml", []byte("[workspace]\nmembers = []\n\n[workspace.package]\nversion = \"9.9.9\"\n"))
	for _, system := range systems {
		name := "lmx-9.9.9-" + system + ".tar.gz"
		data := []byte("archive for " + system)
		digest := sha256.Sum256(data)
		write("dist/"+name, data)
		write("dist/"+name+".sha256", []byte(hex.EncodeToString(digest[:])+"  "+name+"\n"))
	}
	return source
}

func TestLocalArchivesReplaceThePinnedRelease(t *testing.T) {
	source, root := checkout(t), t.TempDir()
	output := filepath.Join(root, "internal", "nixos", "resources", "lmx")
	if err := os.MkdirAll(output, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(output, "lmx-0.0.1-aarch64-linux.tar.gz"), []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}

	var diagnostics bytes.Buffer
	if status := run(context.Background(), []string{"--source", source, "--root", root}, &diagnostics); status != 0 {
		t.Fatalf("status %d: %s", status, diagnostics.String())
	}
	data, err := os.ReadFile(filepath.Join(output, "lmx.json"))
	if err != nil {
		t.Fatal(err)
	}
	var record pin
	if err = json.Unmarshal(data, &record); err != nil {
		t.Fatal(err)
	}
	if record.Version != "9.9.9" || len(record.Systems) != len(systems) {
		t.Fatalf("lmx.json: %s", data)
	}
	for _, system := range systems {
		archive := record.Systems[system]
		copied, err := os.ReadFile(filepath.Join(output, archive.File))
		digest := sha256.Sum256(copied)
		if err != nil || archive.File != "lmx-9.9.9-"+system+".tar.gz" || hex.EncodeToString(digest[:]) != archive.SHA256 {
			t.Fatalf("%s: %+v, %v", system, archive, err)
		}
	}
	if _, err = os.Stat(filepath.Join(output, "lmx-0.0.1-aarch64-linux.tar.gz")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("stale archive kept: %v", err)
	}

	diagnostics.Reset()
	if status := run(context.Background(), []string{"--root", root}, &diagnostics); status != 0 {
		t.Fatalf("status %d: %s", status, diagnostics.String())
	}
	if _, err = os.Stat(output); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("a build without --source kept the local lmx: %v", err)
	}
}

func TestDamagedOrMissingArchivesKeepThePreviousBundle(t *testing.T) {
	root := t.TempDir()
	output := filepath.Join(root, "internal", "nixos", "resources", "lmx")
	if status := run(context.Background(), []string{"--source", checkout(t), "--root", root}, &bytes.Buffer{}); status != 0 {
		t.Fatal("initial bundle failed")
	}
	previous, err := os.ReadFile(filepath.Join(output, "lmx.json"))
	if err != nil {
		t.Fatal(err)
	}

	damaged := checkout(t)
	if err = os.WriteFile(filepath.Join(damaged, "dist", "lmx-9.9.9-x86_64-linux.tar.gz"), []byte("changed"), 0o644); err != nil {
		t.Fatal(err)
	}
	missing := checkout(t)
	if err = os.Remove(filepath.Join(missing, "dist", "lmx-9.9.9-aarch64-linux.tar.gz")); err != nil {
		t.Fatal(err)
	}
	unversioned := checkout(t)
	if err = os.WriteFile(filepath.Join(unversioned, "Cargo.toml"), []byte("[workspace]\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	for source, want := range map[string]string{
		damaged:     "does not match",
		missing:     "task release/build",
		unversioned: "workspace.package.version",
	} {
		var diagnostics bytes.Buffer
		if status := run(context.Background(), []string{"--source", source, "--root", root}, &diagnostics); status != 1 ||
			!strings.Contains(diagnostics.String(), want) {
			t.Fatalf("status %d: %s", status, diagnostics.String())
		}
		if current, err := os.ReadFile(filepath.Join(output, "lmx.json")); err != nil || !bytes.Equal(current, previous) {
			t.Fatalf("a failed bundle replaced the previous one: %v", err)
		}
	}

	if status := run(context.Background(), []string{"--root", root, "extra"}, &bytes.Buffer{}); status != 2 {
		t.Fatalf("positional argument accepted: %d", status)
	}
}
