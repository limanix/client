package modulegen

import (
	"bytes"
	"context"
	"errors"
	"log"
	"os"
	"path/filepath"
	"testing"

	"github.com/limanix/client/internal/nixos/catalog"
)

const imageManifest = `{
  "version": "v4",
  "systems": {
    "aarch64-linux": {
      "url": "https://github.com/limanix/modules/releases/download/v4/limanix-aarch64.qcow2",
      "sha256": "10ecc1ab08c31c48f67106d9fdbb483340a52d963a945aa5c7a91905a1f68417"
    },
    "x86_64-linux": {
      "url": "https://github.com/limanix/modules/releases/download/v4/limanix-x86_64.qcow2",
      "sha256": "1bcc06b3c8f855c80ffd3df0ded38737728b5472494de98921fdb37d55613b24"
    }
  }
}
`

func imageFixture(t *testing.T) string {
	t.Helper()
	manifest := filepath.Join(t.TempDir(), "image.json")
	if err := os.WriteFile(manifest, []byte(imageManifest), 0o644); err != nil {
		t.Fatal(err)
	}
	return manifest
}

func TestLocalImageManifestIsPublishedValidatedAndKept(t *testing.T) {
	directory := t.TempDir()
	published := filepath.Join(directory, "image.json")
	var diagnostics bytes.Buffer
	logger := log.New(&diagnostics, "", 0)
	if err := publishLocalImage(directory, "", logger); !errors.Is(err, catalog.ErrImage) {
		t.Fatalf("missing manifest accepted without a source: %v", err)
	}
	if err := publishLocalImage(directory, imageFixture(t), logger); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(published)
	if err != nil || string(data) != imageManifest {
		t.Fatalf("manifest not published: %v", err)
	}
	if err := publishLocalImage(directory, "", logger); err != nil {
		t.Fatalf("prepared manifest not kept: %v", err)
	}
	invalid := filepath.Join(t.TempDir(), "image.json")
	if err := os.WriteFile(invalid, []byte(`{"version":"v4","systems":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := publishLocalImage(directory, invalid, logger); !errors.Is(err, catalog.ErrImage) {
		t.Fatalf("invalid manifest accepted: %v", err)
	}
	if data, err := os.ReadFile(published); err != nil || string(data) != imageManifest {
		t.Fatalf("invalid manifest replaced the published one: %v", err)
	}
}

func TestPreparedImageManifestIsReusedWithoutDownload(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "image.json"), []byte(imageManifest), 0o644); err != nil {
		t.Fatal(err)
	}
	var diagnostics bytes.Buffer
	logger := log.New(&diagnostics, "", 0)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := prepareImage(ctx, directory, "v4", logger); err != nil {
		t.Fatalf("prepared manifest triggered a download: %v", err)
	}
	if err := prepareImage(ctx, directory, "v5", logger); !errors.Is(err, context.Canceled) {
		t.Fatalf("another release must be downloaded: %v", err)
	}
}
