package catalog

import (
	"errors"
	"strings"
	"testing"
)

const imageFixture = `{
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

func TestParseImage(t *testing.T) {
	for _, tag := range []string{"", "v4"} {
		image, err := ParseImage([]byte(imageFixture), tag)
		if err != nil || image.Version != "v4" || len(image.Systems) != 2 {
			t.Fatalf("tag %q: %+v, %v", tag, image, err)
		}
	}
	if image, _ := ParseImage([]byte(imageFixture), ""); image.Systems["x86_64-linux"].SHA256 != "1bcc06b3c8f855c80ffd3df0ded38737728b5472494de98921fdb37d55613b24" {
		t.Fatalf("x86_64 disk: %+v", image.Systems["x86_64-linux"])
	}
}

func TestParseImageRejectsOtherReleasesAndMalformedManifests(t *testing.T) {
	for name, test := range map[string]struct {
		manifest string
		tag      string
	}{
		"other tag":        {imageFixture, "v5"},
		"invalid JSON":     {"{", ""},
		"trailing data":    {imageFixture + "{}", ""},
		"unknown field":    {strings.Replace(imageFixture, `"version"`, `"catalog": "v4", "version"`, 1), ""},
		"empty version":    {strings.Replace(imageFixture, `"v4"`, `""`, 1), ""},
		"missing system":   {strings.Replace(imageFixture, `"x86_64-linux"`, `"riscv64-linux"`, 1), ""},
		"other repository": {strings.Replace(imageFixture, "limanix/modules", "limanix/images", 1), ""},
		"other release":    {strings.Replace(imageFixture, "download/v4/limanix-aarch64", "download/v3/limanix-aarch64", 1), ""},
		"other file name":  {strings.Replace(imageFixture, "limanix-x86_64.qcow2", "limanix-v4-x86_64.qcow2", 1), ""},
		"uppercase digest": {strings.Replace(imageFixture, "10ecc1ab", "10ECC1AB", 1), ""},
		"short digest":     {strings.Replace(imageFixture, "10ecc1ab", "", 1), ""},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseImage([]byte(test.manifest), test.tag); !errors.Is(err, ErrImage) {
				t.Fatalf("manifest accepted: %v", err)
			}
		})
	}
}
