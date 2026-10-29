package modulegen

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func BenchmarkPackLocal(b *testing.B) {
	source := b.TempDir()
	for _, name := range []string{"LICENSE", "flake.lock", "interface.nix", "platform.nix"} {
		if err := os.WriteFile(filepath.Join(source, name), []byte("fixture\n"), 0o644); err != nil {
			b.Fatal(err)
		}
	}
	content := bytes.Repeat([]byte("{ config, pkgs, ... }: { environment.systemPackages = [ pkgs.hello ]; }\n"), 64)
	for index := range 64 {
		directory := filepath.Join(source, "catalog", "modules", fmt.Sprintf("tool-%02d", index))
		if err := os.MkdirAll(directory, 0o700); err != nil {
			b.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(directory, "default.nix"), content, 0o644); err != nil {
			b.Fatal(err)
		}
	}
	b.ReportAllocs()
	b.SetBytes(int64(len(content) * 64))
	for b.Loop() {
		if _, err := packLocal(context.Background(), source, "benchmark"); err != nil {
			b.Fatal(err)
		}
	}
}
