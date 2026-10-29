package catalog

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"slices"
)

// Repository identifies the standard catalog source in download URLs and ZIP comments.
const Repository = "github.com/limanix/modules"

var tagPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._+-]*$`)

// Kind is the catalog directory that holds a module, and with it how limanix.toml chooses the module.
type Kind string

const (
	// KindShell holds the components of the [shell] presets.
	KindShell Kind = "shell"
	// KindToolchains holds the languages of the [toolchains] table.
	KindToolchains Kind = "toolchains"
	// KindModules holds the modules that [nixos] modules selects by name.
	KindModules Kind = "modules"
)

// Kinds lists the catalog's module directories in the order the client presents them.
var Kinds = []Kind{KindShell, KindToolchains, KindModules}

// Entry describes one selector: the module it belongs to, its line, if any, and its kind.
type Entry struct {
	Kind        Kind
	Module      string
	Line        string
	Description string
}

// Catalog retains validated archive contents and an independent metadata index.
type Catalog struct {
	Version string

	files              fs.FS
	modules            map[string]selection
	nixpkgs            nixpkgsPin
	nixosLima          limaPin
	publicInterface    []byte
	platform           []byte
	publicDeclarations []string
}

// Module exposes the complete catalog tree and the selected NixOS entry point within it.
type Module struct {
	fs.FS
	EntryPoint string
}

// ValidateVersion checks a release tag before it is used in an archive URL.
func ValidateVersion(version string) error {
	if !tagPattern.MatchString(version) {
		return fmt.Errorf("%w: %q", ErrVersion, version)
	}

	return nil
}

// Nixpkgs returns the declared input URL and an independent copy of its locked node.
func (catalog *Catalog) Nixpkgs() (url string, node []byte) {
	return catalog.nixpkgs.url, bytes.Clone(catalog.nixpkgs.node)
}

// NixosLima returns the declared nixos-lima input and independent copies of the lock nodes of its graph,
// which follows the catalog's nixpkgs pin.
func (catalog *Catalog) NixosLima() (Input, map[string]json.RawMessage) {
	nodes := make(map[string]json.RawMessage, len(catalog.nixosLima.nodes))
	for name, node := range catalog.nixosLima.nodes {
		nodes[name] = bytes.Clone(node)
	}
	return catalog.nixosLima.input, nodes
}

// Interface returns an independent copy of the public NixOS option declarations required by every guest.
func (catalog *Catalog) Interface() []byte {
	return bytes.Clone(catalog.publicInterface)
}

// Platform returns an independent copy of the guest platform module that the base disk and every rebuilt guest share.
func (catalog *Catalog) Platform() []byte {
	return bytes.Clone(catalog.platform)
}

// Source returns the complete catalog source tree, rooted at the module directories.
func (catalog *Catalog) Source() fs.FS {
	return catalog.files
}

// PublicDeclarations returns a sorted, independent list of public capability files.
func (catalog *Catalog) PublicDeclarations() []string {
	return slices.Clone(catalog.publicDeclarations)
}

// Modules returns a copy of the local selectors and their entries.
func (catalog *Catalog) Modules() map[string]Entry {
	result := make(map[string]Entry, len(catalog.modules))
	for name, module := range catalog.modules {
		result[name] = module.entry
	}

	return result
}

// Module resolves a local selector while retaining sibling modules for Nix imports.
func (catalog *Catalog) Module(name string) (Module, error) {
	selected, exists := catalog.modules[name]
	if !exists {
		return Module{}, fmt.Errorf("%w: %q", ErrModule, name)
	}

	return Module{FS: catalog.files, EntryPoint: path.Join(selected.directory, selected.entryPoint)}, nil
}
