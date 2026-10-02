package vm

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/limanix/client/internal/domain"
	"github.com/limanix/client/internal/filesystem"
)

func TestGenerationOperationsRejectRedirectedParentAndPreserveOtherContents(t *testing.T) {
	for _, operation := range []struct {
		name string
		run  func(fixtureData, domain.Instance, string) error
	}{
		{"prepare", func(f fixtureData, instance domain.Instance, _ string) error {
			_, err := f.manager.generations.prepare(context.Background(), instance, f.configuration("sandbox"))
			return err
		}},
		{"discard", func(f fixtureData, instance domain.Instance, _ string) error {
			return f.manager.generations.discard(instance)
		}},
		{"prune", func(f fixtureData, instance domain.Instance, _ string) error {
			return errors.Join(f.manager.generations.prune(instance)...)
		}},
		{"update", func(f fixtureData, _ domain.Instance, path string) error {
			_, err := f.manager.Update(context.Background(), path)
			return err
		}},
	} {
		t.Run(operation.name, func(t *testing.T) {
			f := fixture(t)
			instance, path := f.create(t, "sandbox")
			generation, err := f.store.GenerationDir(instance)
			if err != nil {
				t.Fatal(err)
			}
			parent := filepath.Dir(generation)
			if err := os.Rename(parent, parent+"-original"); err != nil {
				t.Fatal(err)
			}
			redirected := t.TempDir()
			sentinels := []string{
				filepath.Join(redirected, instance.Generation, "keep.txt"),
				filepath.Join(redirected, "unrelated", "keep.txt"),
			}
			for _, sentinel := range sentinels {
				if err := os.MkdirAll(filepath.Dir(sentinel), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(sentinel, []byte("unrelated fixture content"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.Symlink(redirected, parent); err != nil {
				t.Fatal(err)
			}
			if err := operation.run(f, instance, path); !errors.Is(err, filesystem.ErrDirectorySymlink) {
				t.Errorf("operation followed redirected generation parent: %v", err)
			}
			for _, sentinel := range sentinels {
				content, err := os.ReadFile(sentinel)
				if err != nil || string(content) != "unrelated fixture content" {
					t.Errorf("unrelated fixture data changed: %s: %v", sentinel, err)
				}
			}
		})
	}
}
