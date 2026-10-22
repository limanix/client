package modules

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/limanix/client/internal/config"
	"github.com/limanix/client/internal/domain"
	"github.com/limanix/client/internal/filesystem"
	"github.com/limanix/client/internal/nixos/catalog"
	"github.com/limanix/client/internal/state"
	"golang.org/x/sys/unix"
)

func fixture(t *testing.T) (*Registry, string) {
	t.Helper()
	root := t.TempDir()
	return registryFixture(t, root), sourceFixture(t, root)
}

func registryFixture(t *testing.T, root string) *Registry {
	t.Helper()
	store, err := state.NewStore(filepath.Join(root, "state"))
	if err != nil {
		t.Fatal(err)
	}
	return NewRegistry(store, map[string]catalog.Entry{
		"docker": {Kind: catalog.KindModules, Module: "docker", Description: "Docker"},
		"rust":   {Kind: catalog.KindToolchains, Module: "rust", Description: "Rust"},
		"neovim": {Kind: catalog.KindShell, Module: "neovim", Description: "Neovim"},
	})
}

// selected lists ids as [nixos] modules chooses them.
func selected(ids ...domain.ModuleID) []config.Choice {
	choices := make([]config.Choice, len(ids))
	for index, id := range ids {
		choices[index] = config.Choice{ID: id, Field: fmt.Sprintf("nixos.modules[%d]", index)}
	}
	return choices
}

func sourceFixture(t *testing.T, root string) string {
	t.Helper()
	source := filepath.Join(root, "module")
	if err := os.MkdirAll(filepath.Join(source, "nested"), 0o700); err != nil {
		t.Fatal(err)
	}
	for path, content := range map[string]string{"default.nix": "{ imports = [ ./nested/extra.nix ]; }", "nested/extra.nix": "{ environment.systemPackages = []; }"} {
		if err := os.WriteFile(filepath.Join(source, path), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return source
}

func TestVersionSelectionDoesNotCreateState(t *testing.T) {
	root := filepath.Join(t.TempDir(), "state")
	store, err := state.NewStore(root)
	if err != nil {
		t.Fatal(err)
	}
	registry := NewRegistry(store, map[string]catalog.Entry{
		"docker":    {Kind: catalog.KindModules, Module: "docker", Description: "Docker (default: 28)"},
		"docker-27": {Kind: catalog.KindModules, Module: "docker", Line: "27", Description: "Docker 27"},
		"docker-28": {Kind: catalog.KindModules, Module: "docker", Line: "28", Description: "Docker 28"},
		"postgres":  {Kind: catalog.KindModules, Module: "postgres", Description: "PostgreSQL"},
	})
	for _, choices := range [][]config.Choice{
		selected("lmx:docker", "lmx:docker-27", "lmx:docker-28"),
		selected("lmx:docker-27", "lmx:docker-27"),
		selected("lmx:postgres", "lmx:docker-28"),
	} {
		if err := registry.Check(context.Background(), choices); err != nil {
			t.Fatal(err)
		}
	}
	for _, id := range []domain.ModuleID{"lmx:docker-99", "lmx:postgres-17", "lmx:unknown"} {
		err = registry.Check(context.Background(), selected(id))
		if !errors.Is(err, ErrUnknownSystem) || !strings.Contains(err.Error(), string(id)) {
			t.Fatalf("wrong unknown-selector diagnostic: %v", err)
		}
	}
	if _, err := os.Stat(root); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("selection check created state: %v", err)
	}
}

func TestImportSourcesAndRemoveAreIndependentOfOriginal(t *testing.T) {
	registry, source := fixture(t)
	ctx := context.Background()
	if err := registry.Add(ctx, domain.ModuleName("custom"), source); err != nil {
		t.Fatal(err)
	}
	if err := registry.Add(ctx, domain.ModuleName("custom"), source); err == nil {
		t.Fatal("replaced an existing import")
	}
	if err := os.RemoveAll(source); err != nil {
		t.Fatal(err)
	}
	set, err := registry.Sources(ctx, selected("lmx:docker", "third-party:custom"))
	if err != nil {
		t.Fatal(err)
	}
	if len(set.Sources) != 2 || set.Sources[0].Path != "" || set.Sources[1].Path == "" {
		t.Fatalf("sources %+v", set.Sources)
	}
	data, err := os.ReadFile(filepath.Join(set.Sources[1].Path, "nested", "extra.nix"))
	if err != nil || string(data) != "{ environment.systemPackages = []; }" {
		t.Fatalf("source snapshot %q: %v", data, err)
	}
	if err := set.Close(); err != nil {
		t.Fatal(err)
	}
	entries, err := registry.Available(ctx)
	if err != nil || len(entries) != 4 || entries[3].Name != "third-party:custom" {
		t.Fatalf("catalog %+v: %v", entries, err)
	}
	if err := registry.Remove(ctx, domain.ModuleName("custom")); err != nil {
		t.Fatal(err)
	}
	if err := registry.Remove(ctx, domain.ModuleName("custom")); err == nil {
		t.Fatal("removed an absent import")
	}
	entries, err = registry.Available(ctx)
	if err != nil || len(entries) != 3 {
		t.Fatalf("removed module remains visible: %+v %v", entries, err)
	}
}

func TestCatalogKeepsDamagedEntriesAsRows(t *testing.T) {
	registry, source := fixture(t)
	ctx := context.Background()
	if err := registry.Add(ctx, domain.ModuleName("valid"), source); err != nil {
		t.Fatal(err)
	}
	directory := filepath.Join(registry.store.Root(), "modules")
	for _, name := range []string{"broken", "BadName"} {
		if err := os.Mkdir(filepath.Join(directory, name), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(directory, "ordinary"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(directory, ".import-in-progress"), 0o700); err != nil {
		t.Fatal(err)
	}
	entries, err := registry.Available(ctx)
	if err != nil || len(entries) != 7 {
		t.Fatalf("one bad directory broke catalog: %+v %v", entries, err)
	}
	for _, entry := range entries[3:6] {
		if entry.Error == nil {
			t.Fatalf("bad entry has no error: %+v", entry)
		}
	}
	if entries[6].Name != "third-party:valid" || entries[6].Error != nil {
		t.Fatalf("healthy module lost: %+v", entries[6])
	}
}

func TestSourcesKeepReadLockUntilCopyCompletes(t *testing.T) {
	registry, source := fixture(t)
	ctx := context.Background()
	if err := registry.Add(ctx, domain.ModuleName("custom"), source); err != nil {
		t.Fatal(err)
	}
	set, err := registry.Sources(ctx, selected("third-party:custom"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := set.Close(); err != nil {
			t.Error(err)
		}
	}()
	// Listing and source lookup are both readers and must coexist.
	if _, err := registry.Available(ctx); err != nil {
		t.Fatal("listing blocked a source reader:", err)
	}
	other, err := registry.Sources(ctx, selected("third-party:custom"))
	if err != nil {
		t.Fatal("source reader blocked another reader:", err)
	}
	if err := other.Close(); err != nil {
		t.Fatal(err)
	}
	limited, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()
	if err := registry.Remove(limited, domain.ModuleName("custom")); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("writer did not wait for held source set: %v", err)
	}
	if _, err := os.Stat(set.Sources[0].Path); err != nil {
		t.Fatal("writer changed locked source:", err)
	}
	if err := set.Close(); err != nil {
		t.Fatal(err)
	}
	if err := registry.Remove(ctx, domain.ModuleName("custom")); err != nil {
		t.Fatal("writer failed after releasing sources:", err)
	}
}

func TestSourcesValidateIdentifiersAndReleaseLockOnFailure(t *testing.T) {
	registry := registryFixture(t, t.TempDir())
	ctx := context.Background()
	for _, value := range []domain.ModuleID{"lmx:unknown", "sys:git", "git", "work:git", "third-party:missing", "../escape", "third-party:../escape"} {
		if _, err := registry.Sources(ctx, selected(value)); err == nil {
			t.Fatalf("accepted source %q", value)
		}
		lock, err := registry.store.RegistryLock(ctx, false, 0)
		if err != nil {
			t.Fatal("failed lookup leaked reader lock:", err)
		}
		if err := lock.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCopyTreeRejectsSymlinksFIFOsAndNestedDestination(t *testing.T) {
	for _, invalid := range []string{"link", "fifo", "entry", "nested"} {
		t.Run(invalid, func(t *testing.T) {
			root := t.TempDir()
			source := sourceFixture(t, root)
			destination := filepath.Join(root, "snapshot")
			switch invalid {
			case "link":
				if err := os.Symlink(source, filepath.Join(source, "link")); err != nil {
					t.Fatal(err)
				}
			case "fifo":
				if err := unix.Mkfifo(filepath.Join(source, "fifo"), 0o600); err != nil {
					t.Fatal(err)
				}
			case "entry":
				entry := filepath.Join(source, "default.nix")
				if err := os.Remove(entry); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(filepath.Join(source, "nested", "extra.nix"), entry); err != nil {
					t.Fatal(err)
				}
			case "nested":
				destination = filepath.Join(source, "snapshot")
			}
			if _, err := CopyTree(source, destination); err == nil {
				t.Fatal("accepted invalid source tree")
			}
			if _, err := os.Stat(destination); !errors.Is(err, fs.ErrNotExist) {
				t.Fatalf("failed clone left a destination: %v", err)
			}
			if invalid != "nested" {
				registry := registryFixture(t, root)
				if err := registry.Add(context.Background(), domain.ModuleName("bad"), source); err == nil {
					t.Fatal("accepted invalid imported tree")
				}
			}
		})
	}
}

func TestCopyTreePreservesRelativeImportsAndExistingDestination(t *testing.T) {
	root := t.TempDir()
	source := sourceFixture(t, root)
	destination, err := filesystem.Resolve(filepath.Join(root, "snapshot"))
	if err != nil {
		t.Fatal(err)
	}
	entry, err := CopyTree(source, destination)
	if err != nil || entry != filepath.Join(destination, "default.nix") {
		t.Fatalf("entry %q: %v", entry, err)
	}
	if _, err := os.Stat(filepath.Join(destination, "nested", "extra.nix")); err != nil {
		t.Fatal("lost nested import:", err)
	}
	if _, err := CopyTree(source, destination); err == nil {
		t.Fatal("overwrote an existing snapshot")
	}
	if _, err := os.Stat(entry); err != nil {
		t.Fatal("removed an existing snapshot on failed copy:", err)
	}
}

func TestCopyTreeRejectsDanglingDestinationSymlink(t *testing.T) {
	root := t.TempDir()
	source := sourceFixture(t, root)
	destination := filepath.Join(root, "snapshot")
	target := filepath.Join(root, "outside")
	if err := os.Symlink(target, destination); err != nil {
		t.Fatal(err)
	}
	if _, err := CopyTree(source, destination); err == nil {
		t.Fatal("followed a dangling destination symlink")
	}
	if _, err := os.Stat(target); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("created dangling link target: %v", err)
	}
	info, err := os.Lstat(destination)
	if err != nil || info.Mode()&fs.ModeSymlink == 0 {
		t.Fatalf("changed destination symlink: %v", err)
	}
}

func TestConcurrentImportsCommitExactlyOneCompleteTree(t *testing.T) {
	registry, source := fixture(t)
	results := make(chan error, 2)
	start := make(chan struct{})
	for range 2 {
		go func() {
			<-start
			results <- registry.Add(context.Background(), domain.ModuleName("same"), source)
		}()
	}
	close(start)
	successes := 0
	for range 2 {
		if err := <-results; err == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("committed %d imports, want exactly one", successes)
	}
	directory := filepath.Join(registry.store.Root(), "modules")
	entries, err := os.ReadDir(directory)
	if err != nil || len(entries) != 1 || entries[0].Name() != "same" {
		t.Fatalf("partial import directories leaked: %v %v", entries, err)
	}
	if _, err := os.Stat(filepath.Join(directory, "same", "nested", "extra.nix")); err != nil {
		t.Fatal("committed incomplete tree:", err)
	}
}

func TestImportPreparesUnderSourceReadLockAndWaitsToCommit(t *testing.T) {
	registry, source := fixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := registry.Add(ctx, domain.ModuleName("custom"), source); err != nil {
		cancel()
		t.Fatal(err)
	}
	set, err := registry.Sources(ctx, selected("third-party:custom"))
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	result := make(chan error, 1)
	finished := false
	defer func() {
		cancel()
		if err := set.Close(); err != nil {
			t.Error(err)
		}
		if !finished {
			if err := <-result; err != nil {
				t.Log(err)
			}
		}
	}()
	go func() { result <- registry.Add(ctx, domain.ModuleName("second"), source) }()

	// Observe the prepared tree while a source reader still owns the shared lock.
	// The exclusive commit cannot publish the import until that reader releases it.
	directory := filepath.Join(registry.store.Root(), "modules")
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	prepared := false
	for !prepared {
		select {
		case err := <-result:
			finished = true
			t.Fatalf("import finished before the source reader released its lock: %v", err)
		case <-ctx.Done():
			t.Fatal("import did not prepare while the source reader held its lock:", ctx.Err())
		case <-ticker.C:
			entries, err := os.ReadDir(directory)
			if err != nil {
				t.Fatal(err)
			}
			for _, entry := range entries {
				if !strings.HasPrefix(entry.Name(), ".second.import-") {
					continue
				}
				path := filepath.Join(directory, entry.Name(), "module", "nested", "extra.nix")
				data, err := os.ReadFile(path)
				if err != nil && !errors.Is(err, fs.ErrNotExist) {
					t.Fatal(err)
				}
				prepared = err == nil && string(data) == "{ environment.systemPackages = []; }"
			}
		}
	}
	if _, err := os.Stat(filepath.Join(directory, "second")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("import was published while a source reader held its lock: %v", err)
	}
	if err := set.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-result:
		finished = true
		if err != nil {
			t.Fatal("import did not commit after the reader released its lock:", err)
		}
	case <-ctx.Done():
		t.Fatal("import did not finish after the reader released its lock:", ctx.Err())
	}
	if _, err := os.Stat(filepath.Join(directory, "second", "nested", "extra.nix")); err != nil {
		t.Fatal("committed import is incomplete:", err)
	}
}

func TestNixOSModulesChooseOnlyModulesOfKindModules(t *testing.T) {
	store, err := state.NewStore(filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	registry := NewRegistry(store, map[string]catalog.Entry{
		"go":      {Kind: catalog.KindToolchains, Module: "go", Description: "Go (default: 1.10)"},
		"go-1.9":  {Kind: catalog.KindToolchains, Module: "go", Line: "1.9"},
		"go-1.10": {Kind: catalog.KindToolchains, Module: "go", Line: "1.10"},
		"rust":    {Kind: catalog.KindToolchains, Module: "rust"},
		"tmux":    {Kind: catalog.KindShell, Module: "tmux"},
		"zsh":     {Kind: catalog.KindShell, Module: "zsh"},
	})
	ctx := context.Background()
	for id, want := range map[domain.ModuleID]string{
		"lmx:go":     `nixos.modules[0]: lmx:go is a toolchain; choose its version in [toolchains], such as go = "1.10"`,
		"lmx:go-1.9": `nixos.modules[0]: lmx:go-1.9 is a toolchain; choose its version in [toolchains], such as go = "1.9"`,
		"lmx:rust":   `nixos.modules[0]: lmx:rust is a toolchain; choose its version in [toolchains], such as rust = "stable"`,
		"lmx:tmux":   `nixos.modules[0]: lmx:tmux is part of the shell; choose it in [shell]: multiplexer = "tmux"`,
		"lmx:zsh":    `nixos.modules[0]: lmx:zsh is part of the shell; choose it in [shell]: preset = "terminal"`,
	} {
		if err := registry.Check(ctx, selected(id)); !errors.Is(err, ErrOtherKind) || err.Error() != want {
			t.Fatalf("%s: got %v, want %s", id, err, want)
		}
	}

	tables := []config.Choice{
		{ID: "lmx:go-1.9", Field: "toolchains.go"},
		{ID: "lmx:rust", Field: "toolchains.rust"},
		{ID: "lmx:tmux", Field: "shell.multiplexer"},
		{ID: "lmx:zsh", Field: "shell.preset"},
	}
	if err := registry.Check(ctx, tables); err != nil {
		t.Fatalf("the tables that choose them were refused: %v", err)
	}

	err = registry.Check(ctx, []config.Choice{{ID: "lmx:go-1.8", Field: "toolchains.go"}})
	want := `toolchains.go: the bundled catalog has no go line "1.8"; choose one of 1.9, 1.10`
	if !errors.Is(err, ErrUnknownSystem) || err == nil || err.Error() != want {
		t.Fatalf("unknown line: got %v, want %s", err, want)
	}
}
