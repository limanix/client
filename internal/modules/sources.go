package modules

import (
	"context"
	"errors"
	"io"
	"path/filepath"
	"strings"

	"github.com/limanix/client/internal/config"
	"github.com/limanix/client/internal/domain"
	"github.com/limanix/client/internal/nixos/catalog"
	"github.com/limanix/client/internal/state"
)

// Source references an embedded module by ID or a stable imported directory by Path.
type Source struct {
	ID   domain.ModuleID
	Path string
}

// SourceSet holds the registry's shared lock until all sources have been copied into a generation.
type SourceSet struct {
	Sources []Source
	lock    io.Closer
}

// Close releases source stability; imported paths must not be used after it returns.
func (sources *SourceSet) Close() error {
	return sources.lock.Close()
}

// Check performs preliminary selection checks without creating state or acquiring locks.
func (r *Registry) Check(ctx context.Context, choices []config.Choice) error {
	for _, choice := range choices {
		if err := ctx.Err(); err != nil {
			return err
		}

		if _, err := r.source(choice); err != nil {
			return err
		}
	}

	return ctx.Err()
}

// Sources holds a shared registry lock while resolving selected source trees.
func (r *Registry) Sources(ctx context.Context, choices []config.Choice) (*SourceSet, error) {
	lock, err := r.store.RegistryLock(ctx, true, state.RegistryLockTimeout)
	if err != nil {
		return nil, err
	}

	result := &SourceSet{
		Sources: make([]Source, 0, len(choices)),
		lock:    lock,
	}

	for _, choice := range choices {
		source, err := r.source(choice)
		if err != nil {
			return nil, errors.Join(err, result.Close())
		}

		result.Sources = append(result.Sources, source)
	}

	return result, nil
}

func (r *Registry) source(choice config.Choice) (Source, error) {
	id, err := domain.NewModuleID(string(choice.ID))
	if err != nil {
		return Source{}, err
	}

	source := Source{ID: id}
	if id.Namespace() == "lmx" {
		entry, exists := r.system[id.Selector()]
		if !exists {
			if key, isToolchain := strings.CutPrefix(choice.Field, "toolchains."); isToolchain {
				return Source{}, r.unknownLine(choice, key)
			}

			return Source{}, &Error{
				ID:  id,
				Err: ErrUnknownSystem,
			}
		}

		if strings.HasPrefix(choice.Field, "nixos.modules") && entry.Kind != catalog.KindModules {
			return Source{}, r.otherKind(choice, entry)
		}

		return source, nil
	}

	if !id.IsThirdParty() {
		return Source{}, &Error{ID: id, Err: ErrUnknownCatalog}
	}

	name, err := domain.NewModuleName(id.Selector())
	if err != nil {
		return Source{}, &Error{ID: id, Err: err}
	}

	source.Path = filepath.Join(r.directory(), string(name))
	if err = ValidateDirectory(source.Path); err != nil {
		return Source{}, &Error{
			ID:  id,
			Err: err,
		}
	}

	return source, nil
}
