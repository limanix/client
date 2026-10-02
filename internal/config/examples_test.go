package config

import (
	"path/filepath"
	"testing"
)

func TestDocumentedConfigurationExamplesLoad(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	examples, err := filepath.Glob("../../guides/examples/*.toml")
	if err != nil || len(examples) == 0 {
		t.Fatalf("configuration examples are missing: %v", err)
	}
	for _, example := range examples {
		t.Run(filepath.Base(example), func(t *testing.T) {
			if _, err := Load(example); err != nil {
				t.Fatalf("documented configuration cannot be loaded: %v", err)
			}
		})
	}
}
