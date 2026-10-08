package app

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/limanix/client/internal/bundle"
	"github.com/limanix/client/internal/guest"
	"github.com/limanix/client/internal/lima"
	"github.com/limanix/client/internal/managedhome"
	"github.com/limanix/client/internal/modules"
	"github.com/limanix/client/internal/nixos"
	"github.com/limanix/client/internal/state"
	"github.com/limanix/client/internal/vm"
)

// Services is the lazy composition root for one CLI invocation.
type Services struct {
	input       io.Reader
	output      io.Writer
	diagnostics io.Writer

	store func() (*state.Store, error)
}

// New retains streams without initializing host services.
func New(input io.Reader, output, diagnostics io.Writer) *Services {
	return &Services{
		input:       input,
		output:      output,
		diagnostics: diagnostics,

		store: sync.OnceValues(
			func() (*state.Store, error) {
				return state.NewStore("")
			},
		),
	}
}

// Manager assembles VM lifecycle services after checking the native architecture.
func (s *Services) Manager() (*vm.Manager, error) {
	host, err := lima.HostArchitecture()
	if err != nil {
		return nil, err
	}

	if err = lima.RequireNativeArchitecture(host); err != nil {
		return nil, err
	}

	store, err := s.store()
	if err != nil {
		return nil, err
	}

	registry, err := s.Registry()
	if err != nil {
		return nil, err
	}

	var (
		agents  = bundle.New(filepath.Join(store.Root(), "runtime", "guestagents"))
		backend = lima.NewClient(agents.Path)
	)

	backend.Stdin = s.input
	backend.Stdout = s.output
	backend.Stderr = s.diagnostics

	var (
		guests = guest.New(backend)
		warn   = log.New(s.diagnostics, "limanix: warning: ", 0).Printf
	)

	guests.Stdout = s.output
	guests.Stderr = s.diagnostics
	guests.Warn = warn

	manager := vm.New(vm.Dependencies{
		Store:   store,
		Backend: backend,
		Modules: registry,
		Homes:   &managedhome.Manager{},
		Guest:   guests,
		HostUID: os.Getuid(),
	})
	manager.Warn = warn

	return manager, nil
}

// Registry opens module services without initializing Lima or guest access.
func (s *Services) Registry() (*modules.Registry, error) {
	store, err := s.store()
	if err != nil {
		return nil, err
	}

	metadata, err := nixos.SystemModules()
	if err != nil {
		return nil, err
	}

	return modules.NewRegistry(store, metadata), nil
}
