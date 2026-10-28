package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestReferenceJSONMatchesTheCommandsAndConfiguration(t *testing.T) {
	reference, err := BuildReference()
	if err != nil {
		t.Fatal(err)
	}
	var encoded bytes.Buffer
	encoder := json.NewEncoder(&encoded)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err = encoder.Encode(reference); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join("..", "..", "reference.json")
	if os.Getenv("LIMANIX_UPDATE_REFERENCE") != "" {
		if err = os.WriteFile(path, encoded.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	saved, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(saved, encoded.Bytes()) {
		t.Fatalf("reference.json is out of date: run task reference/fix modules_version=<tag> (%v)", err)
	}
}
