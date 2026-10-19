package modulegen

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"path/filepath"

	"github.com/limanix/client/internal/filesystem"
	"github.com/limanix/client/internal/nixos/catalog"
)

// prepareImage publishes the base image manifest of the catalog release, reusing a valid one.
func prepareImage(ctx context.Context, directory, version string, logger *log.Logger) error {
	filename := filepath.Join(directory, "image.json")
	if data, err := os.ReadFile(filename); err == nil {
		if _, err = catalog.ParseImage(data, version); err == nil {
			logger.Printf("%s image %s is already prepared.", catalog.Repository, version)
			return nil
		}
	}

	data, err := downloadImage(ctx, version)
	if err != nil {
		return err
	}
	if _, err = catalog.ParseImage(data, version); err != nil {
		return err
	}
	if err = filesystem.WriteFileAtomic(filename, data, 0o644); err != nil {
		return fmt.Errorf("publish base image manifest: %w", err)
	}

	logger.Printf("Prepared %s image %s.", catalog.Repository, version)
	return nil
}

func downloadImage(ctx context.Context, version string) (data []byte, failure error) {
	address := "https://" + catalog.Repository + "/releases/download/" + url.PathEscape(version) + "/image.json"
	response, err := get(ctx, address)
	if err != nil {
		return nil, fmt.Errorf("download %s image %s: %w", catalog.Repository, version, err)
	}
	defer func() {
		failure = errors.Join(failure, response.Body.Close())
	}()

	if data, err = io.ReadAll(io.LimitReader(response.Body, 1<<20)); err != nil {
		return nil, fmt.Errorf("download %s image %s: %w", catalog.Repository, version, err)
	}
	return data, nil
}

// publishLocalImage publishes the manifest at source, or keeps a valid prepared one when source is empty.
func publishLocalImage(directory, source string, logger *log.Logger) error {
	filename := filepath.Join(directory, "image.json")
	previous, err := os.ReadFile(filename)
	if source == "" {
		if err == nil {
			_, err = catalog.ParseImage(previous, "")
		}
		if err != nil {
			return fmt.Errorf("%w: a local catalog needs --image with a release's image.json: %w", catalog.ErrImage, err)
		}
		logger.Printf("Kept the prepared base image manifest.")
		return nil
	}

	data, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	if _, err = catalog.ParseImage(data, ""); err != nil {
		return err
	}
	if bytes.Equal(previous, data) {
		logger.Printf("Local base image manifest is already prepared.")
		return nil
	}
	if err = filesystem.WriteFileAtomic(filename, data, 0o644); err != nil {
		return fmt.Errorf("publish base image manifest: %w", err)
	}

	logger.Printf("Prepared base image manifest from %s.", source)
	return nil
}
