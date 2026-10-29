package catalog

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/limanix/client/internal/domain"
	"github.com/pelletier/go-toml/v2"
)

// metadata declares optional version selectors; package sources remain entirely in Nix.
type metadata struct {
	Description string   `toml:"description"`
	Default     *string  `toml:"default"`
	Versions    []string `toml:"versions"`
}

// selection binds a public local identifier to files under one physical module directory.
type selection struct {
	directory  string
	entryPoint string
	entry      Entry
}

func readMetadata(files fs.FS, directory string) (metadata, error) {
	var result metadata

	data, err := fs.ReadFile(files, path.Join(directory, "module.toml"))
	if err != nil {
		return result, fmt.Errorf("%w: module.toml: %w", ErrMetadata, err)
	}

	if !utf8.Valid(data) {
		return result, fmt.Errorf("%w: module.toml must be UTF-8", ErrMetadata)
	}

	decoder := toml.NewDecoder(bytes.NewReader(data)).DisallowUnknownFields()
	if err = decoder.Decode(&result); err != nil {
		return result, fmt.Errorf("%w: module.toml: %w", ErrMetadata, err)
	}

	if err = result.validate(); err != nil {
		return result, err
	}

	entries := []string{"default.nix"}
	for _, version := range result.Versions {
		entries = append(entries, versionEntry(version))
	}

	for _, name := range entries {
		entry, err := fs.Stat(files, path.Join(directory, name))
		if err != nil {
			return result, fmt.Errorf("%w: %s: %w", ErrMetadata, name, err)
		}

		if !entry.Mode().IsRegular() {
			return result, fmt.Errorf("%w: %s must be a regular file", ErrMetadata, name)
		}
	}

	return result, nil
}

func (metadata metadata) validate() error {
	if strings.TrimSpace(metadata.Description) == "" {
		return fmt.Errorf("%w: description must not be empty", ErrMetadata)
	}

	if len(metadata.Versions) == 0 {
		if metadata.Default != nil {
			return fmt.Errorf("%w: default requires a nonempty versions list", ErrMetadata)
		}

		return nil
	}

	if metadata.Default == nil || !slices.Contains(metadata.Versions, *metadata.Default) {
		return fmt.Errorf("%w: default must select a declared version", ErrMetadata)
	}

	seen := make(map[string]bool, len(metadata.Versions))
	for _, version := range metadata.Versions {
		if err := domain.ValidateModuleVersion(version); err != nil {
			return fmt.Errorf("%w: version %q: %w", ErrMetadata, version, err)
		}

		if seen[version] {
			return fmt.Errorf("%w: duplicate version %q", ErrMetadata, version)
		}

		seen[version] = true
	}

	return nil
}

func (metadata metadata) selections(kind Kind, name string) map[string]selection {
	directory := path.Join(string(kind), name)
	base := selection{
		directory:  directory,
		entryPoint: "default.nix",
		entry:      Entry{Kind: kind, Module: name, Description: metadata.Description},
	}
	if metadata.Default != nil {
		base.entry.Description += " (default: " + *metadata.Default + ")"
	}

	result := map[string]selection{name: base}
	for _, version := range metadata.Versions {
		result[name+"-"+version] = selection{
			directory:  directory,
			entryPoint: versionEntry(version),
			entry: Entry{
				Kind:        kind,
				Module:      name,
				Line:        version,
				Description: metadata.Description + " (version: " + version + ")",
			},
		}
	}

	return result
}

func versionEntry(version string) string {
	return path.Join("versions", version+".nix")
}

// readModules indexes modules/<kind>/<name>; below modules/ there are only _shared, the kinds and regular files.
func readModules(files fs.FS) (map[string]selection, error) {
	entries, err := fs.ReadDir(files, "modules")
	if err != nil {
		return nil, fmt.Errorf("%w: read modules: %w", ErrMetadata, err)
	}

	result := make(map[string]selection)

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
		kind := Kind(name)
		if !slices.Contains(Kinds, kind) || !entry.IsDir() {
			return nil, fmt.Errorf("%w: expected _shared or a kind directory (shell, toolchains, modules), got %q", ErrMetadata, name)
		}
		if err = readKind(files, kind, result); err != nil {
			return nil, err
		}
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("%w: no modules found", ErrMetadata)
	}

	return result, nil
}

// readKind adds the selectors of one kind's modules; selectors are unique across kinds.
func readKind(files fs.FS, kind Kind, result map[string]selection) error {
	directory := path.Join("modules", string(kind))
	entries, err := fs.ReadDir(files, directory)
	if err != nil {
		return fmt.Errorf("%w: read %s: %w", ErrMetadata, directory, err)
	}

	for _, entry := range entries {
		name := entry.Name()
		if entry.Type().IsRegular() {
			continue
		}
		if name == "capabilities" || name == "internal" || name == "pins" || name == "_shared" {
			return fmt.Errorf("%w: module name %q is reserved", ErrMetadata, name)
		}
		if _, err = domain.NewModuleName(name); err != nil || !entry.IsDir() {
			return fmt.Errorf("%w: expected a module directory in %s, got %q", ErrMetadata, kind, name)
		}

		metadata, err := readMetadata(files, path.Join(directory, name))
		if err != nil {
			return fmt.Errorf("module %q: %w", name, err)
		}

		for selector, selected := range metadata.selections(kind, name) {
			if _, exists := result[selector]; exists {
				return fmt.Errorf("%w: duplicate selector %q", ErrMetadata, selector)
			}

			result[selector] = selected
		}
	}

	return nil
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
