package catalog

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"slices"
	"strings"

	"github.com/limanix/client/internal/domain"
)

// Repository identifies the standard catalog source in download URLs and ZIP comments.
const Repository = "github.com/limanix/modules"

var tagPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._+-]*$`)

// Catalog retains validated archive contents and an independent metadata index.
type Catalog struct {
	Version string

	files              fs.FS
	modules            map[string]selection
	nixpkgs            nixpkgsPin
	publicInterface    []byte
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

// Open validates a complete catalog without extracting it to the host filesystem.
func Open(data []byte) (*Catalog, error) {
	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrArchive, err)
	}

	if archive.Comment != Repository {
		return nil, fmt.Errorf("%w: got %q, expected %q", ErrRepository, archive.Comment, Repository)
	}

	if err = validateArchive(archive); err != nil {
		return nil, err
	}

	version, err := fs.ReadFile(archive, "version")
	if err != nil {
		return nil, fmt.Errorf("%w: read version: %w", ErrArchive, err)
	}

	tag := strings.TrimSpace(string(version))
	if err = ValidateVersion(tag); err != nil {
		return nil, err
	}

	if _, err = fs.ReadFile(archive, "LICENSE"); err != nil {
		return nil, fmt.Errorf("%w: read LICENSE: %w", ErrArchive, err)
	}

	interfaceInfo, err := fs.Stat(archive, "interface.nix")
	if err != nil {
		return nil, fmt.Errorf("%w: read interface.nix: %w", ErrArchive, err)
	}
	if !interfaceInfo.Mode().IsRegular() {
		return nil, fmt.Errorf("%w: interface.nix must be a regular file", ErrArchive)
	}
	publicInterface, err := fs.ReadFile(archive, "interface.nix")
	if err != nil {
		return nil, fmt.Errorf("%w: read interface.nix: %w", ErrArchive, err)
	}
	if len(publicInterface) == 0 {
		return nil, fmt.Errorf("%w: interface.nix must not be empty", ErrArchive)
	}

	pin, err := readNixpkgs(archive)
	if err != nil {
		return nil, err
	}

	metadata, err := readModules(archive)
	if err != nil {
		return nil, err
	}

	source, err := fs.Sub(archive, "modules")
	if err != nil {
		return nil, fmt.Errorf("%w: read module sources: %w", ErrArchive, err)
	}
	declarations, err := readPublicDeclarations(source)
	if err != nil {
		return nil, err
	}

	return &Catalog{
		Version: tag, files: source, modules: metadata, nixpkgs: pin,
		publicInterface: publicInterface, publicDeclarations: declarations,
	}, nil
}

// Nixpkgs returns the declared input URL and an independent copy of its locked node.
func (catalog *Catalog) Nixpkgs() (url string, node []byte) {
	return catalog.nixpkgs.url, bytes.Clone(catalog.nixpkgs.node)
}

// Interface returns an independent copy of the public NixOS option declarations required by every guest.
func (catalog *Catalog) Interface() []byte {
	return bytes.Clone(catalog.publicInterface)
}

// Source returns the complete catalog source tree, rooted at the module directories.
func (catalog *Catalog) Source() fs.FS {
	return catalog.files
}

// PublicDeclarations returns a sorted, independent list of public capability files.
func (catalog *Catalog) PublicDeclarations() []string {
	return slices.Clone(catalog.publicDeclarations)
}

// Modules returns a copy of the local module names and their descriptions.
func (catalog *Catalog) Modules() map[string]string {
	result := make(map[string]string, len(catalog.modules))
	for name, module := range catalog.modules {
		result[name] = module.description
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

func readModules(files fs.FS) (map[string]selection, error) {
	entries, err := fs.ReadDir(files, "modules")
	if err != nil {
		return nil, fmt.Errorf("%w: read modules: %w", ErrMetadata, err)
	}

	result := make(map[string]selection, len(entries))

	for _, entry := range entries {
		name := entry.Name()
		if name == "_shared" {
			if !entry.IsDir() {
				return nil, fmt.Errorf("%w: _shared must be a directory", ErrMetadata)
			}
			continue
		}
		if entry.Type().IsRegular() {
			continue
		}
		if name == "capabilities" || name == "internal" || name == "pins" {
			return nil, fmt.Errorf("%w: module name %q is reserved", ErrMetadata, name)
		}
		if _, err = domain.NewModuleName(name); err != nil || !entry.IsDir() {
			return nil, fmt.Errorf("%w: expected a module directory, got %q", ErrMetadata, name)
		}

		metadata, err := readMetadata(files, path.Join("modules", name))
		if err != nil {
			return nil, fmt.Errorf("module %q: %w", name, err)
		}

		for selector, selected := range metadata.selections(name) {
			if _, exists := result[selector]; exists {
				return nil, fmt.Errorf("%w: duplicate selector %q", ErrMetadata, selector)
			}

			result[selector] = selected
		}
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("%w: no modules found", ErrMetadata)
	}

	return result, nil
}

// Direct Nix files declare public areas; test.nix belongs to the test ABI.
func readPublicDeclarations(source fs.FS) ([]string, error) {
	entries, err := fs.ReadDir(source, "_shared")
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("%w: read _shared declarations: %w", ErrMetadata, err)
	}

	var result []string
	for _, entry := range entries {
		name := entry.Name()
		if name == "module.toml" {
			return nil, fmt.Errorf("%w: _shared must not contain module.toml", ErrMetadata)
		}
		if name == "test.nix" || !strings.HasSuffix(name, ".nix") {
			continue
		}
		if !entry.Type().IsRegular() {
			return nil, fmt.Errorf("%w: public declaration %q must be a regular file", ErrMetadata, name)
		}
		result = append(result, path.Join("_shared", name))
	}
	return result, nil
}
