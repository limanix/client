package modules

import (
	"maps"
	"path/filepath"
	"slices"

	"github.com/limanix/client/internal/nixos/catalog"
	"github.com/limanix/client/internal/state"
)

// Registry keeps copied third-party source trees separate from their original checkout.
type Registry struct {
	store  *state.Store
	system map[string]catalog.Entry
	themes []string
}

// NewRegistry takes its own copy of the embedded-module metadata and theme names supplied by NixOS.
func NewRegistry(store *state.Store, system map[string]catalog.Entry, themes []string) *Registry {
	return &Registry{
		store:  store,
		system: maps.Clone(system),
		themes: slices.Clone(themes),
	}
}

func (r *Registry) directory() string {
	return filepath.Join(r.store.Root(), "modules")
}
