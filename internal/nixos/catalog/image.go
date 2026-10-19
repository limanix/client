package catalog

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"
)

var digestPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

// Image is a catalog release's image.json: the base disk of each guest system and its SHA-256.
type Image struct {
	Version string          `json:"version"`
	Systems map[string]Disk `json:"systems"`
}

// Disk is one system's base disk: its release asset and its SHA-256.
type Disk struct {
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
}

// ParseImage validates a release's image.json. A nonempty tag must match the manifest's version.
func ParseImage(data []byte, tag string) (Image, error) {
	var image Image
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&image); err != nil {
		return Image{}, fmt.Errorf("%w: %w", ErrImage, err)
	}
	if decoder.More() {
		return Image{}, fmt.Errorf("%w: trailing data after the manifest", ErrImage)
	}
	if err := ValidateVersion(image.Version); err != nil {
		return Image{}, fmt.Errorf("%w: %w", ErrImage, err)
	}
	if tag != "" && image.Version != tag {
		return Image{}, fmt.Errorf("%w: version %q, expected %q", ErrImage, image.Version, tag)
	}
	if systems := slices.Sorted(maps.Keys(image.Systems)); !slices.Equal(systems, []string{"aarch64-linux", "x86_64-linux"}) {
		return Image{}, fmt.Errorf("%w: systems %v, expected aarch64-linux and x86_64-linux", ErrImage, systems)
	}
	for system, disk := range image.Systems {
		want := "https://" + Repository + "/releases/download/" + image.Version + "/limanix-" + strings.TrimSuffix(system, "-linux") + ".qcow2"
		if disk.URL != want {
			return Image{}, fmt.Errorf("%w: %s: url %q, expected %q", ErrImage, system, disk.URL, want)
		}
		if !digestPattern.MatchString(disk.SHA256) {
			return Image{}, fmt.Errorf("%w: %s: sha256 must contain 64 lowercase hexadecimal characters", ErrImage, system)
		}
	}
	return image, nil
}
