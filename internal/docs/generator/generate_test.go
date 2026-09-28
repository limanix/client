package generator

import (
	"context"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestGeneratePreparesCompleteTree(t *testing.T) {
	root := t.TempDir()
	for path, content := range map[string]string{
		"guides/index.md":             "# Client\n",
		"guides/examples/config.toml": "name = \"example\"\n",
		"build/docs/removed.md":       "stale guide",
		"build/docs/generated/old.md": "stale reference",
	} {
		path = filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if err := Generate(context.Background(), root, io.Discard); err != nil {
		t.Fatal(err)
	}

	output := os.DirFS(filepath.Join(root, "build", "docs"))
	var paths []string
	if err := fs.WalkDir(output, ".", func(path string, entry fs.DirEntry, err error) error {
		if err == nil && !entry.IsDir() {
			paths = append(paths, path)
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"examples/config.toml",
		"generated/cli.md",
		"generated/configuration.md",
		"generated/limanix.example.toml",
		"generated/metadata.json",
		"index.md",
	}
	if !reflect.DeepEqual(paths, want) {
		t.Fatalf("prepared files = %v, want %v", paths, want)
	}
	for _, path := range []string{"index.md", "examples/config.toml"} {
		original, err := os.ReadFile(filepath.Join(root, "guides", path))
		if err != nil {
			t.Fatal(err)
		}
		copied, err := fs.ReadFile(output, path)
		if err != nil {
			t.Fatal(err)
		}
		if string(copied) != string(original) {
			t.Fatalf("guide %s changed during preparation", path)
		}
	}
}
