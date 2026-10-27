package nixos

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/limanix/client/internal/config"
	"github.com/limanix/client/internal/domain"
	"github.com/limanix/client/internal/filesystem"
	"github.com/limanix/client/internal/modules"
)

// Prepare copies module trees once into a generation and writes runtime ENV files beside its flake, keeping their
// values out of flake sources and Nix store. The guest learns the generation ID from runtime.json. The development
// account accepts the authorized keys over SSH.
func Prepare(cfg config.Config, runtimeDir, generation string, sources []modules.Source, uid int, authorizedKeys []string) (string, error) {
	if uid <= 0 {
		return "", ErrInvalidUID
	}

	for _, key := range authorizedKeys {
		if key == "" || strings.ContainsAny(key, "\r\n") {
			return "", ErrInvalidAuthorizedKey
		}
	}

	if !domain.ValidIdentifier(generation) {
		return "", ErrInvalidGeneration
	}

	serviceEnvironment, shellEnvironment, err := renderEnvironmentFiles(cfg.Env)
	if err != nil {
		return "", err
	}

	flakeDir := filepath.Join(runtimeDir, "flake")
	if err = requireNewBundle(flakeDir); err != nil {
		return "", err
	}

	if err = validateSources(sources); err != nil {
		return "", err
	}

	if err = filesystem.CheckDirectory(runtimeDir); err != nil {
		return "", err
	}

	if err = os.MkdirAll(runtimeDir, 0o700); err != nil {
		return "", err
	}

	if err = copyResource("resources/base", flakeDir); err != nil {
		return "", err
	}
	if err = overlayLMX(resources, flakeDir); err != nil {
		return "", err
	}
	if err = writeFlake(flakeDir); err != nil {
		return "", err
	}

	imports, err := copyModules(flakeDir, sources)
	if err != nil {
		return "", err
	}

	if err = writeRuntime(filepath.Join(flakeDir, "runtime.json"), cfg, generation, imports, uid, authorizedKeys); err != nil {
		return "", err
	}

	if err = writeEnvironmentFiles(runtimeDir, serviceEnvironment, shellEnvironment); err != nil {
		return "", err
	}

	return flakeDir, nil
}

// overlayLMX copies the local lmx archives and their lmx.json that a development build embeds over the pinned
// release; see cmd/bundle-lmx.
func overlayLMX(files fs.FS, flakeDir string) error {
	if _, err := fs.Stat(files, "resources/lmx/lmx.json"); errors.Is(err, fs.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}

	local, err := fs.Sub(files, "resources/lmx")
	if err != nil {
		return err
	}

	return copyFiles(local, flakeDir)
}

func requireNewBundle(directory string) error {
	_, err := os.Lstat(directory)

	switch {
	case err == nil:
		return fmt.Errorf("%w: %s", ErrBundleExists, directory)
	case errors.Is(err, fs.ErrNotExist):
		return nil
	default:
		return err
	}
}
