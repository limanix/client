package modules

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"strconv"
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

// otherKind names the table that chooses a shell component or a toolchain listed in [nixos] modules.
func (r *Registry) otherKind(choice config.Choice, entry catalog.Entry) error {
	var message string
	if entry.Kind == catalog.KindShell {
		message = fmt.Sprintf("%s is part of the shell; choose it in [shell]: %s", choice.ID, config.ShellSetting(entry.Module))
	} else {
		key, versioned := config.ToolchainKey(entry.Module)
		example := `"stable"`
		if lines := r.lines(entry.Module); versioned && len(lines) > 0 {
			example = strconv.Quote(lines[len(lines)-1])
		}
		if entry.Line != "" {
			example = strconv.Quote(entry.Line)
		}
		message = fmt.Sprintf("%s is a toolchain; choose its version in [toolchains], such as %s = %s", choice.ID, key, example)
	}

	return &config.Error{Field: choice.Field, Message: message, Cause: ErrOtherKind}
}

// unknownLine lists the lines that the bundled catalog offers for a toolchain.
func (r *Registry) unknownLine(choice config.Choice, key string) error {
	module, line := choice.ID.Selector(), ""
	if index := strings.LastIndexByte(module, '-'); index >= 0 {
		module, line = module[:index], module[index+1:]
	}
	lines := r.lines(module)
	message := fmt.Sprintf("the bundled catalog has no %s line %q", key, line)
	if len(lines) > 0 {
		message += "; choose one of " + strings.Join(lines, ", ")
	}

	return &config.Error{Field: choice.Field, Message: message, Cause: ErrUnknownSystem}
}

// lines returns the declared lines of a bundled module, oldest first.
func (r *Registry) lines(module string) []string {
	var lines []string
	for _, entry := range r.system {
		if entry.Module == module && entry.Line != "" {
			lines = append(lines, entry.Line)
		}
	}

	slices.SortFunc(lines, compareLines)
	return lines
}

// compareLines orders numeric lines such as 1.9 and 1.10 by their components.
func compareLines(a, b string) int {
	left, right := strings.Split(a, "."), strings.Split(b, ".")
	for index := range min(len(left), len(right)) {
		x, _ := strconv.Atoi(left[index])
		y, _ := strconv.Atoi(right[index])
		if x != y {
			return x - y
		}
	}

	return len(left) - len(right)
}
