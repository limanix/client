package generator

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/limanix/client/internal/filesystem"
)

// Generate copies guides and writes the generated references into root/build/docs.
func Generate(ctx context.Context, root string, diagnostics io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	root, err := filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("resolve repository root: %w", err)
	}

	guides := filepath.Join(root, "guides")
	index, err := os.Stat(filepath.Join(guides, "index.md"))
	if err != nil {
		return fmt.Errorf("read guides/index.md: %w", err)
	}
	if !index.Mode().IsRegular() {
		return fmt.Errorf("guides/index.md: %w", filesystem.ErrNotRegular)
	}

	documents, err := render()
	if err != nil {
		return err
	}

	output := filepath.Join(root, "build", "docs")
	for _, directory := range []string{filepath.Dir(output), output} {
		if err = filesystem.CheckDirectory(directory); err != nil {
			return fmt.Errorf("prepare documentation directory: %w", err)
		}
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	if err = os.RemoveAll(output); err != nil {
		return fmt.Errorf("remove previous documentation: %w", err)
	}
	if err = os.CopyFS(output, os.DirFS(guides)); err != nil {
		return fmt.Errorf("copy guides: %w", err)
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	if err = os.Mkdir(filepath.Join(output, "generated"), 0o755); err != nil {
		return fmt.Errorf("prepare generated documentation directory: %w", err)
	}

	for _, document := range documents {
		if err = ctx.Err(); err != nil {
			return err
		}

		if err = filesystem.WriteFileAtomic(filepath.Join(root, document.path), document.content, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", document.path, err)
		}
	}

	log.New(diagnostics, "build-docs: ", 0).Printf("Prepared documentation in %s.\n", output)
	return nil
}
