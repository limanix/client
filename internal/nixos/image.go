package nixos

import (
	"fmt"

	"github.com/limanix/client/internal/domain"
	"github.com/limanix/client/internal/nixos/catalog"
)

// Image identifies the base disk that Lima downloads and verifies before first boot.
type Image struct {
	Location string
	SHA256   string
}

// BaseImage selects the disk for the guest architecture from resources/image.json, the manifest of the
// bundled catalog release.
//
// The disk's partition layout and boot loader are the catalog's platform.nix, which every guest imports.
func BaseImage(architecture domain.Architecture) (Image, error) {
	arch, err := architecture.LimaArch()
	if err != nil {
		return Image{}, err
	}

	manifest, err := imageManifest()
	if err != nil {
		return Image{}, err
	}

	disk, found := manifest.Systems[arch+"-linux"]
	if !found {
		return Image{}, fmt.Errorf("%w: %s", ErrImagePin, arch)
	}

	return Image{Location: disk.URL, SHA256: disk.SHA256}, nil
}

// BaseImageLocations lists the disks of all guest systems in the bundled catalog release.
func BaseImageLocations() ([]string, error) {
	manifest, err := imageManifest()
	if err != nil {
		return nil, err
	}

	locations := make([]string, 0, len(manifest.Systems))
	for _, disk := range manifest.Systems {
		locations = append(locations, disk.URL)
	}

	return locations, nil
}

func imageManifest() (catalog.Image, error) {
	data, err := resources.ReadFile("resources/image.json")
	if err != nil {
		return catalog.Image{}, err
	}

	manifest, err := catalog.ParseImage(data, "")
	if err != nil {
		return catalog.Image{}, fmt.Errorf("read base image manifest: %w", err)
	}

	return manifest, nil
}
