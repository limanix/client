package modulegen

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/limanix/client/internal/filesystem"
	"github.com/limanix/client/internal/nixos/catalog"
	"golang.org/x/sys/unix"
)

// GenerateLocal packages the distributable files of a local catalog checkout, with the base image
// manifest at image or the prepared one when image is empty.
// A label does not cache local source: every call validates its current contents.
// Identical archive bytes preserve the existing output, including its timestamp.
func GenerateLocal(ctx context.Context, root, source, version, image string, diagnostics io.Writer) error {
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
	} else {
		if err := filesystem.WriteFileAtomic(filename, data, 0o644); err != nil {
			return fmt.Errorf("publish local module catalog: %w", err)
		}
		logger.Printf("Prepared local catalog %s from %s.", version, source)
	}
	return publishLocalImage(directory, image, logger)
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
	writer := zip.NewWriter(&buffer)
	if err := writer.SetComment(catalog.Repository); err != nil {
		return nil, err
	}

	for _, name := range []string{"LICENSE", "flake.lock", "interface.nix", "platform.nix", "catalog"} {
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
			name := filepath.ToSlash(relative)
			if !fs.ValidPath(name) || strings.Contains(name, `\`) {
				return fmt.Errorf("%w: unsafe source path %q", catalog.ErrArchive, "modules-local/"+name)
			}
			if module, ok := strings.CutPrefix(name, "catalog/"); ok {
				name = "modules/" + module
			} else if name != "LICENSE" && name != "flake.lock" && name != "interface.nix" && name != "platform.nix" {
				return nil
			}
			return writeLocalFile(writer, path, name, info)
		})
		if err != nil {
			return nil, errors.Join(err, writer.Close())
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := finishArchive(writer, version); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func writeLocalFile(writer *zip.Writer, path, name string, info fs.FileInfo) (failure error) {
	input, err := filesystem.OpenRegular(path, unix.O_RDONLY, 0)
	if err != nil {
		return err
	}
	defer func() {
		failure = errors.Join(failure, input.Close())
	}()

	header := &zip.FileHeader{Name: name, Method: zip.Deflate}
	header.SetMode(info.Mode().Perm())
	output, err := writer.CreateHeader(header)
	if err != nil {
		return err
	}

	// Require the copied byte count to match the inspected size.
	written, err := io.Copy(output, io.LimitReader(input, info.Size()+1))
	if err != nil {
		return err
	}
	if written != info.Size() {
		return fmt.Errorf("source file %q changed size while being packaged", path)
	}
	return nil
}
