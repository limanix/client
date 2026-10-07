package nixos

import (
	"encoding/json"

	"github.com/limanix/client/internal/config"
	"github.com/limanix/client/internal/domain"
	"github.com/limanix/client/internal/filesystem"
)

type runtimeUser struct {
	Name domain.Username  `json:"name"`
	Home domain.GuestPath `json:"home"`
	Sudo bool             `json:"sudo"`
	UID  int              `json:"uid"`
}

// runtimeDisk carries the configured size as a plain byte count for Nix arithmetic.
type runtimeDisk struct {
	Bytes          int64  `json:"bytes"`
	CollectPercent uint64 `json:"collectPercent"`
	MinimumPercent uint64 `json:"minimumPercent"`
}

type runtimeConfig struct {
	Name            domain.VMName       `json:"name"`
	Arch            domain.Architecture `json:"arch"`
	Generation      string              `json:"generation"`
	User            runtimeUser         `json:"user"`
	Disk            runtimeDisk         `json:"disk"`
	Ports           config.Ports        `json:"ports"`
	Modules         []string            `json:"modules"`
	SelectedModules []domain.ModuleID   `json:"selectedModules"`
}

func writeRuntime(filename string, cfg config.Config, generation string, imports []string, uid int) error {
	record := runtimeConfig{
		Name:       cfg.Name,
		Arch:       cfg.Resources.Arch,
		Generation: generation,
		User: runtimeUser{
			Name: cfg.User.Name,
			Home: cfg.User.Home,
			Sudo: cfg.User.Sudo,
			UID:  uid,
		},
		Disk: runtimeDisk{
			Bytes:          int64(cfg.Resources.Disk),
			CollectPercent: domain.DiskCollectPercent,
			MinimumPercent: domain.DiskMinimumPercent,
		},
		Ports:           cfg.Network.Ports,
		Modules:         imports,
		SelectedModules: append([]domain.ModuleID{}, cfg.NixOS.Modules...),
	}

	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}

	return filesystem.WriteFileAtomic(filename, append(data, '\n'), 0o600)
}
