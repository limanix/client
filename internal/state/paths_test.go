package state

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/limanix/client/internal/domain"
	"github.com/limanix/client/internal/filesystem"
)

func TestStoreRejectsRedirectedRootAfterConstruction(t *testing.T) {
	for _, operation := range []struct {
		name string
		run  func(*Store, domain.Instance) error
	}{
		{"instance path", func(store *Store, instance domain.Instance) error {
			_, err := store.InstanceDir(instance.Identity.Name)
			return err
		}},
		{"load identity", func(store *Store, instance domain.Instance) error {
			_, err := store.LoadIdentity(instance.Identity.Name)
			return err
		}},
		{"load", func(store *Store, instance domain.Instance) error {
			_, err := store.Load(instance.Identity.Name)
			return err
		}},
		{"list", func(store *Store, _ domain.Instance) error {
			_, err := store.FetchAll()
			return err
		}},
		{"remove", func(store *Store, instance domain.Instance) error {
			return store.Remove(instance.Identity.Name)
		}},
		{"forget home", func(store *Store, instance domain.Instance) error {
			return store.ForgetHome(instance.Identity)
		}},
	} {
		t.Run(operation.name, func(t *testing.T) {
			store, original := fixture(t, "sandbox")
			if err := store.Save(original); err != nil {
				t.Fatal(err)
			}
			homeRecord, err := store.PreserveHome(original.Identity)
			if err != nil {
				t.Fatal(err)
			}
			preservedRoot := store.Root() + "-original"
			if err := os.Rename(store.Root(), preservedRoot); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(preservedRoot, store.Root()); err != nil {
				t.Fatal(err)
			}
			if err := operation.run(store, original); !errors.Is(err, filesystem.ErrDirectorySymlink) {
				t.Fatalf("operation followed redirected root: %v", err)
			}
			for _, path := range []string{
				filepath.Join(preservedRoot, "instances", string(original.Identity.Name), "identity.json"),
				filepath.Join(preservedRoot, "homes", filepath.Base(homeRecord)),
			} {
				if _, err := os.Stat(path); err != nil {
					t.Fatalf("redirected record changed: %v", err)
				}
			}
		})
	}
}
