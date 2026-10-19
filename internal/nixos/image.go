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

	data, err := resources.ReadFile("resources/image.json")
	if err != nil {
		return Image{}, err
	}

	manifest, err := catalog.ParseImage(data, "")
	if err != nil {
		return Image{}, fmt.Errorf("read base image manifest: %w", err)
	}

	disk, found := manifest.Systems[arch+"-linux"]
	if !found {
		return Image{}, fmt.Errorf("%w: %s", ErrImagePin, arch)
	}

	return Image{Location: disk.URL, SHA256: disk.SHA256}, nil
}
