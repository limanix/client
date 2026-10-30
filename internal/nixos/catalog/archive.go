package catalog

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path"
	"strings"
)

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

	publicInterface, err := readRequiredModule(archive, "interface.nix")
	if err != nil {
		return nil, err
	}

	platform, err := readRequiredModule(archive, "platform.nix")
	if err != nil {
		return nil, err
	}

	pin, err := readNixpkgs(archive)
	if err != nil {
		return nil, err
	}

	lima, err := readNixosLima(archive)
	if err != nil {
		return nil, err
	}

	metadata, err := readModules(archive)
	if err != nil {
		return nil, err
	}

	themes, err := readThemes(archive)
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
		Version:            tag,
		files:              source,
		modules:            metadata,
		themes:             themes,
		nixpkgs:            pin,
		nixosLima:          lima,
		publicInterface:    publicInterface,
		platform:           platform,
		publicDeclarations: declarations,
	}, nil
}

func readRequiredModule(files fs.FS, name string) ([]byte, error) {
	info, err := fs.Stat(files, name)
	if err != nil {
		return nil, fmt.Errorf("%w: read %s: %w", ErrArchive, name, err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%w: %s must be a regular file", ErrArchive, name)
	}
	data, err := fs.ReadFile(files, name)
	if err != nil {
		return nil, fmt.Errorf("%w: read %s: %w", ErrArchive, name, err)
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("%w: %s must not be empty", ErrArchive, name)
	}
	return data, nil
}

func validateArchive(archive *zip.Reader) error {
	entries := make(map[string]fs.FileMode, len(archive.File))

	for _, file := range archive.File {
		name := strings.TrimSuffix(file.Name, "/")
		mode := file.Mode()

		switch {
		case !fs.ValidPath(name) || name == "." || strings.Contains(name, `\`):
			return fmt.Errorf("%w: unsafe path %q", ErrArchive, file.Name)
		case !mode.IsRegular() && !mode.IsDir():
			return fmt.Errorf("%w: unsupported file %q", ErrArchive, file.Name)
		}

		if _, exists := entries[name]; exists {
			return fmt.Errorf("%w: duplicate path %q", ErrArchive, name)
		}
		entries[name] = mode

		if mode.IsRegular() && name != "interface.nix" && name != "platform.nix" {
			if err := checkFile(file); err != nil {
				return fmt.Errorf("%w: %s: %w", ErrArchive, name, err)
			}
		}
	}

	for name := range entries {
		for parent := path.Dir(name); parent != "."; parent = path.Dir(parent) {
			if mode, exists := entries[parent]; exists && !mode.IsDir() {
				return fmt.Errorf("%w: file %q is also used as a directory", ErrArchive, parent)
			}
		}
	}

	return nil
}

func checkFile(file *zip.File) error {
	reader, err := file.Open()
	if err != nil {
		return err
	}

	_, err = io.Copy(io.Discard, reader)
	return errors.Join(err, reader.Close())
}
