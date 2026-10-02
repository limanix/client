package modulegen

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/limanix/client/internal/nixos/catalog"
	"golang.org/x/sys/unix"
)

func localFixture(t *testing.T) string {
	t.Helper()
	source := t.TempDir()
	for name, data := range catalogFiles(t) {
		filename := filepath.Join(source, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(filename), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filename, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return source
}

func TestLocalCatalogIsDeterministicAndContainsOnlyDistributableFiles(t *testing.T) {
	source := localFixture(t)
	if err := os.WriteFile(filepath.Join(source, "README.md"), []byte("not shipped\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	before, err := packLocal(context.Background(), source, "local-20261001")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := catalog.Open(before); err != nil {
		t.Fatal(err)
	}
	timestamp := time.Unix(100, 0)
	if err := os.Chtimes(filepath.Join(source, "interface.nix"), timestamp, timestamp); err != nil {
		t.Fatal(err)
	}
	after, err := packLocal(context.Background(), source, "local-20261001")
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("file timestamps changed archive bytes: %v", err)
	}
	archive, err := zip.NewReader(bytes.NewReader(after), int64(len(after)))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fs.ReadFile(archive, "README.md"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("repository docs shipped: %v", err)
	}
	for _, name := range []string{"LICENSE", "flake.lock", "interface.nix", "modules/tool/default.nix", "modules/tool/module.toml", "version"} {
		if _, err := fs.ReadFile(archive, name); err != nil {
			t.Fatalf("missing %s: %v", name, err)
		}
	}
}

func TestLocalCatalogRejectsSymlinksSpecialFilesAndCanceledWork(t *testing.T) {
	for _, kind := range []string{"symlink", "fifo", "directory-symlink"} {
		t.Run(kind, func(t *testing.T) {
			source := localFixture(t)
			path := filepath.Join(source, "catalog", "tool", "unsafe")
			switch kind {
			case "symlink":
				if err := os.Symlink("default.nix", path); err != nil {
					t.Fatal(err)
				}
			case "fifo":
				if err := unix.Mkfifo(path, 0o600); err != nil {
					t.Fatal(err)
				}
			case "directory-symlink":
				link := filepath.Join(t.TempDir(), "linked")
				if err := os.Symlink(source, link); err != nil {
					t.Fatal(err)
				}
				source = link
			}
			if _, err := packLocal(context.Background(), source, "local"); !errors.Is(err, catalog.ErrArchive) {
				t.Fatalf("unsupported source accepted: %v", err)
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := packLocal(ctx, localFixture(t), "local"); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation lost: %v", err)
	}
}

func TestGenerateLocalRefreshesChangedSourceWithSameLabelAndPreservesValidOutput(t *testing.T) {
	source := localFixture(t)
	root := t.TempDir()
	output := filepath.Join(root, "internal", "nixos", "resources", "modules.zip")
	if err := os.MkdirAll(filepath.Dir(output), 0o700); err != nil {
		t.Fatal(err)
	}
	var diagnostics bytes.Buffer
	generate := func() error { return GenerateLocal(context.Background(), root, source, "local-20261001", &diagnostics) }
	if err := generate(); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	timestamp := time.Unix(100, 0)
	if err := os.Chtimes(output, timestamp, timestamp); err != nil {
		t.Fatal(err)
	}
	if err := generate(); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(output)
	if err != nil || !info.ModTime().Equal(timestamp) {
		t.Fatalf("identical output was rewritten: %v, %v", info, err)
	}
	if err := os.WriteFile(filepath.Join(source, "catalog", "tool", "default.nix"), []byte("{...}: { environment.variables.LOCAL = \"updated\"; }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := generate(); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(output)
	if err != nil || bytes.Equal(before, after) {
		t.Fatalf("same label cached stale source: %v", err)
	}
	if err := os.Remove(filepath.Join(source, "interface.nix")); err != nil {
		t.Fatal(err)
	}
	if err := generate(); err == nil {
		t.Fatal("invalid source accepted")
	}
	retained, err := os.ReadFile(output)
	if err != nil || !bytes.Equal(after, retained) {
		t.Fatalf("invalid source replaced valid output: %v", err)
	}
}
