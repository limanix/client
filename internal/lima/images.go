package lima

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/lima-vm/lima/v2/pkg/downloader"
	"github.com/limanix/client/internal/nixos"
)

const imageOrigin = "https://github.com/limanix/"

// PruneImages removes cached LimaNix base disks of other catalog releases and leaves other Lima downloads alone.
func (client *Client) PruneImages(ctx context.Context) (failure error) {
	defer func() {
		failure = operationError(ctx, "prune images", failure)
	}()

	current, err := nixos.BaseImageLocations()
	if err != nil {
		return err
	}

	return withNetworkLock(ctx, func() error {
		entries, err := downloader.CacheEntries(downloader.WithCache())
		if err != nil {
			return err
		}

		var failures []error
		for _, entry := range entries {
			failures = append(failures, pruneImage(entry, current))
		}

		return errors.Join(failures...)
	})
}

func pruneImage(entry string, current []string) error {
	data, err := os.ReadFile(filepath.Join(entry, "url"))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}

	if err != nil {
		return err
	}

	location := string(data)
	if !strings.HasPrefix(location, imageOrigin) || slices.Contains(current, location) {
		return nil
	}

	return os.RemoveAll(entry)
}
