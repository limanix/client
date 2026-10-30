package nixos

import (
	"encoding/json"

	"github.com/limanix/client/internal/config"
	"github.com/limanix/client/internal/domain"
	"github.com/limanix/client/internal/filesystem"
)

type runtimeUser struct {
	Name           domain.Username  `json:"name"`
	Home           domain.GuestPath `json:"home"`
	Sudo           bool             `json:"sudo"`
	UID            int              `json:"uid"`
	AuthorizedKeys []string         `json:"authorizedKeys"`
}

// runtimeDisk carries the configured size as a plain byte count for Nix arithmetic.
type runtimeDisk struct {
	Bytes          int64  `json:"bytes"`
	CollectPercent uint64 `json:"collectPercent"`
	MinimumPercent uint64 `json:"minimumPercent"`
}

type runtimeTheme struct {
	Name string `json:"name"`
}

// runtimeShell carries the shell settings that modules read; null where the chosen shell has no such setting.
type runtimeShell struct {
	// Prompt of Zsh, null without the terminal preset.
	Prompt *string `json:"prompt"`
	// Dotfiles is the guest mount of shell.dotfiles, null without it.
	Dotfiles *string `json:"dotfiles"`
}

// runtimeToolchains carries the toolchain settings that are not a catalog line.
type runtimeToolchains struct {
	// Rust is the default rustup toolchain, null without Rust.
	Rust *string `json:"rust"`
}

type runtimeConfig struct {
	Name            domain.VMName        `json:"name"`
	Arch            domain.Architecture  `json:"arch"`
	Generation      string               `json:"generation"`
	User            runtimeUser          `json:"user"`
	Disk            runtimeDisk          `json:"disk"`
	Ports           config.Ports         `json:"ports"`
	Theme           runtimeTheme         `json:"theme"`
	Shell           runtimeShell         `json:"shell"`
	Toolchains      runtimeToolchains    `json:"toolchains"`
	Modules         []string             `json:"modules"`
	SelectedModules []domain.ModuleID    `json:"selectedModules"`
	Packages        []domain.PackageName `json:"packages"`
}

func writeRuntime(filename string, cfg config.Config, generation string, imports []string, uid int, authorizedKeys []string) error {
	selection := cfg.Selection()
	selected := make([]domain.ModuleID, len(selection.Modules))
	for index, choice := range selection.Modules {
		selected[index] = choice.ID
	}

	var shell runtimeShell
	if cfg.Shell.Preset == config.PresetTerminal {
		shell.Prompt = &cfg.Shell.Prompt
	}
	if cfg.Shell.Dotfiles != "" {
		shell.Dotfiles = new(config.DotfilesTarget)
	}

	var toolchains runtimeToolchains
	if cfg.Toolchains.Rust != "" {
		toolchains.Rust = new(string(cfg.Toolchains.Rust))
	}

	record := runtimeConfig{
		Name:       cfg.Name,
		Arch:       cfg.Resources.Arch,
		Generation: generation,
		User: runtimeUser{
			Name:           cfg.User.Name,
			Home:           cfg.User.Home,
			Sudo:           cfg.User.Sudo,
			UID:            uid,
			AuthorizedKeys: append([]string{}, authorizedKeys...),
		},
		Disk: runtimeDisk{
			Bytes:          int64(cfg.Resources.Disk),
			CollectPercent: domain.DiskCollectPercent,
			MinimumPercent: domain.DiskMinimumPercent,
		},
		Ports:           cfg.Network.Ports,
		Theme:           runtimeTheme{Name: cfg.Theme.Name},
		Shell:           shell,
		Toolchains:      toolchains,
		Modules:         imports,
		SelectedModules: selected,
		Packages:        append([]domain.PackageName{}, selection.Packages...),
	}

	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}

	return filesystem.WriteFileAtomic(filename, append(data, '\n'), 0o600)
}
