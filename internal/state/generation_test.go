package state

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/limanix/client/internal/filesystem"
)

func TestGenerationDirChecksParentAndLeafWithoutCreatingPaths(t *testing.T) {
	store, instance := fixture(t, "sandbox")
	generation, err := store.GenerationDir(instance)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(store.Root()); !os.IsNotExist(err) {
		t.Fatalf("generation lookup allocated state: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(generation), 0o700); err != nil {
		t.Fatal(err)
	}
	target := t.TempDir()
	if err := os.Symlink(target, generation); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GenerationDir(instance); !errors.Is(err, filesystem.ErrDirectorySymlink) {
		t.Fatalf("redirected generation leaf accepted: %v", err)
	}
	if err := os.Remove(generation); err != nil {
		t.Fatal(err)
	}
	parent := filepath.Dir(generation)
	if err := os.Remove(parent); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, parent); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GenerationDir(instance); !errors.Is(err, filesystem.ErrDirectorySymlink) {
		t.Fatalf("redirected generation parent accepted: %v", err)
	}
}
