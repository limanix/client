package modulegen

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"

	"github.com/limanix/client/internal/filesystem"
	"github.com/limanix/client/internal/nixos/catalog"
	"golang.org/x/sys/unix"
)

// GenerateLocal packages the distributable files of a local catalog checkout.
// A label does not cache local source: every call validates its current contents.
// Identical archive bytes preserve the existing output, including its timestamp.
func GenerateLocal(ctx context.Context, root, source, version string, diagnostics io.Writer) error {
	if err := catalog.ValidateVersion(version); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	directory, err := filesystem.RequireDirectory(filepath.Join(root, "internal", "nixos", "resources"))
	if err != nil {
		return err
	}
	data, err := packLocal(ctx, source, version)
	if err != nil {
		return err
	}
	if _, err := catalog.Open(data); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	filename := filepath.Join(directory, "modules.zip")
	logger := log.New(diagnostics, "bundle-modules: ", 0)
	previous, err := os.ReadFile(filename)
	if err == nil && bytes.Equal(previous, data) {
		logger.Printf("Local catalog %s is already prepared.", version)
		return nil
	}
	if err := filesystem.WriteFileAtomic(filename, data, 0o644); err != nil {
		return fmt.Errorf("publish local module catalog: %w", err)
	}
	logger.Printf("Prepared local catalog %s from %s.", version, source)
	return nil
}

func packLocal(ctx context.Context, source, version string) ([]byte, error) {
	info, err := os.Lstat(source)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() || info.Mode()&fs.ModeSymlink != 0 {
		return nil, fmt.Errorf("%w: local source must be a real directory", catalog.ErrArchive)
	}
	source, err = filepath.Abs(source)
	if err != nil {
		return nil, err
	}
	var buffer bytes.Buffer
	compressed := gzip.NewWriter(&buffer)
	writer := tar.NewWriter(compressed)
	for _, name := range []string{"LICENSE", "flake.lock", "interface.nix", "catalog"} {
		err = filepath.WalkDir(filepath.Join(source, name), func(path string, entry fs.DirEntry, failure error) error {
			if failure != nil {
				return failure
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}
			relative, err := filepath.Rel(source, path)
			if err != nil {
				return err
			}
			if !info.Mode().IsRegular() {
				return fmt.Errorf("%w: unsupported local source file %q", catalog.ErrArchive, relative)
			}
			input, err := filesystem.OpenRegular(path, unix.O_RDONLY, 0)
			if err != nil {
				return err
			}
			header := &tar.Header{
				Name:     "modules-local/" + filepath.ToSlash(relative),
				Mode:     int64(info.Mode().Perm()),
				Size:     info.Size(),
				Typeflag: tar.TypeReg,
			}
			if err := writer.WriteHeader(header); err != nil {
				return errors.Join(err, input.Close())
			}
			_, copyErr := io.Copy(writer, input)
			return errors.Join(copyErr, input.Close())
		})
		if err != nil {
			return nil, errors.Join(err, writer.Close(), compressed.Close())
		}
	}
	if err := errors.Join(writer.Close(), compressed.Close()); err != nil {
		return nil, err
	}
	return pack(ctx, &buffer, version)
}
