package modules

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/limanix/client/internal/domain"
	"github.com/limanix/client/internal/nixos/catalog"
	"github.com/limanix/client/internal/state"
)

// Info is a catalog row, including an error for a damaged imported module. Kind says how limanix.toml chooses a
// bundled module: modules in [nixos] modules, toolchains in [toolchains], shell components in [shell].
type Info struct {
	Name        string  `json:"name"`
	Source      string  `json:"source"`
	Kind        string  `json:"kind"`
	Description string  `json:"description"`
	Error       *string `json:"error"`
}

// Available returns bundled modules and independently reports invalid catalog entries.
func (r *Registry) Available(ctx context.Context) (entries []Info, failure error) {
	for name, entry := range r.system {
		entries = append(entries, Info{
			Name:        "lmx:" + name,
			Source:      "lmx",
			Kind:        string(entry.Kind),
			Description: entry.Description,
		})
	}

	// Modules that [nixos] modules selects come first, then the toolchains, then the shell components.
	order := func(kind string) int {
		return len(catalog.Kinds) - 1 - slices.Index(catalog.Kinds, catalog.Kind(kind))
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Kind != entries[j].Kind {
			return order(entries[i].Kind) < order(entries[j].Kind)
		}
		return entries[i].Name < entries[j].Name
	})

	lock, err := r.store.RegistryLock(ctx, true, state.RegistryLockTimeout)
	if err != nil {
		return nil, err
	}

	defer func() {
		failure = errors.Join(failure, lock.Close())
	}()

	paths, err := os.ReadDir(r.directory())
	if err != nil {
		return nil, fmt.Errorf("cannot list imported modules: %w", err)
	}

	for _, path := range paths {
		if strings.HasPrefix(path.Name(), ".") {
			continue
		}

		entries = append(entries, r.importedInfo(path.Name()))
	}

	return entries, nil
}

func (r *Registry) importedInfo(name string) Info {
	entry := Info{
		Name:        "third-party:" + name,
		Source:      "third-party",
		Kind:        string(catalog.KindModules),
		Description: "Locally imported NixOS module",
	}

	if err := r.validateImport(name); err != nil {
		entry.Error = new(err.Error())
	}

	return entry
}

func (r *Registry) validateImport(name string) error {
	if _, err := domain.NewModuleName(name); err != nil {
		return err
	}

	return ValidateDirectory(filepath.Join(r.directory(), name))
}
