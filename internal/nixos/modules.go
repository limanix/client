package nixos

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"

	"github.com/limanix/client/internal/domain"
	"github.com/limanix/client/internal/modules"
	"github.com/limanix/client/internal/nixos/catalog"
)

// SystemModules reads the embedded catalog and returns a copy of its names and descriptions.
func SystemModules() (map[string]string, error) {
	catalog, err := systemCatalog()
	if err != nil {
		return nil, err
	}

	return catalog.Modules(), nil
}

func validateSources(sources []modules.Source) error {
	for _, source := range sources {
		if _, err := domain.NewModuleID(string(source.ID)); err != nil {
			return fmt.Errorf("invalid module identifier: %w", err)
		}

		if source.Path == "" {
			if _, err := systemModule(source.ID); err != nil {
				return err
			}

			continue
		}

		if !source.ID.IsThirdParty() {
			return fmt.Errorf("module %q: only imported modules may use a local source path", source.ID)
		}

		if err := modules.ValidateDirectory(source.Path); err != nil {
			return err
		}
	}

	return nil
}

func copyModules(flakeDir string, sources []modules.Source) ([]string, error) {
	stored, err := systemCatalog()
	if err != nil {
		return nil, err
	}
	declarations := stored.PublicDeclarations()
	imports := make([]string, 0, len(declarations)+len(sources))
	for _, declaration := range declarations {
		imports = append(imports, path.Join("modules", "lmx", declaration))
	}
	catalogCopied := false
	if len(imports) > 0 || len(sources) > 0 {
		if err := os.MkdirAll(filepath.Join(flakeDir, "modules"), 0o700); err != nil {
			return nil, err
		}
	}

	for index, source := range sources {
		if source.Path != "" {
			name := fmt.Sprintf("%04d", index)
			if _, err := modules.CopyTree(source.Path, filepath.Join(flakeDir, "modules", name)); err != nil {
				return nil, fmt.Errorf("copy module %q: %w", source.ID, err)
			}
			imports = append(imports, path.Join("modules", name, "default.nix"))
			continue
		}

		files, err := systemModule(source.ID)
		if err != nil {
			return nil, err
		}
		if !catalogCopied {
			if err = copyFiles(files, filepath.Join(flakeDir, "modules", "lmx")); err != nil {
				return nil, fmt.Errorf("copy catalog: %w", err)
			}
			catalogCopied = true
		}
		imports = append(imports, path.Join("modules", "lmx", files.EntryPoint))
	}

	if !catalogCopied && len(declarations) > 0 {
		shared, err := fs.Sub(stored.Source(), "_shared")
		if err != nil {
			return nil, fmt.Errorf("read shared catalog declarations: %w", err)
		}
		if err = copyFiles(shared, filepath.Join(flakeDir, "modules", "lmx", "_shared")); err != nil {
			return nil, fmt.Errorf("copy shared catalog declarations: %w", err)
		}
	}

	return imports, nil
}

func systemModule(id domain.ModuleID) (catalog.Module, error) {
	if id.Namespace() != "lmx" {
		return catalog.Module{}, fmt.Errorf("module %q: an embedded source requires the lmx namespace", id)
	}

	stored, err := systemCatalog()
	if err != nil {
		return catalog.Module{}, err
	}

	return stored.Module(id.Selector())
}
