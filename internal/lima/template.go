package lima

import (
	"encoding/json"
	"net"
	"sort"
	"strings"

	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/limanix/client/internal/config"
	"github.com/limanix/client/internal/domain"
	"github.com/limanix/client/internal/nixos"
	"github.com/opencontainers/go-digest"
)

// Render uses Lima's schema types, emitting JSON (a YAML subset) without secret ENV values.
func Render(cfg config.Config, managedHome, runtimeDir string, hostArch domain.Architecture, hostUID, sshPort int) ([]byte, error) {
	if _, err := domain.NewArchitecture(string(hostArch)); err != nil {
		return nil, err
	}

	if sshPort <= 0 || sshPort > 65535 {
		return nil, ErrSSHPort
	}

	arch, err := cfg.Resources.Arch.LimaArch()
	if err != nil {
		return nil, err
	}

	memory, err := cfg.Resources.Mem.GiB()
	if err != nil {
		return nil, err
	}

	disk, err := cfg.Resources.Disk.GiB()
	if err != nil {
		return nil, err
	}

	if hostUID <= 0 || managedHome == "" || runtimeDir == "" {
		return nil, ErrManagedMounts
	}

	image, err := nixos.BaseImage(cfg.Resources.Arch)
	if err != nil {
		return nil, err
	}

	document := machineTemplate(cfg.Resources.Arch, hostArch)
	document.Arch = &arch
	document.Images = []limatype.Image{{
		File: limatype.File{
			Location: image.Location,
			Arch:     arch,
			Digest:   digest.NewDigestFromEncoded(digest.SHA256, image.SHA256),
		},
	}}
	document.CPUs = new(cfg.Resources.CPU)
	document.Memory = &memory
	document.Disk = &disk
	document.User = managementUser(hostUID)
	document.Mounts = templateMounts(cfg, managedHome, runtimeDir)
	document.SSH.LocalPort = &sshPort

	encoded, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return nil, err
	}

	return append(encoded, '\n'), nil
}

func usesVZ(architecture, host domain.Architecture) bool {
	switch architecture {
	case domain.ARM64, domain.AMD64:
		return architecture == host
	default:
		return false
	}
}

func machineTemplate(architecture, host domain.Architecture) limatype.LimaYAML {
	document := limatype.LimaYAML{
		VMType:    new(limatype.QEMU),
		MountType: new(limatype.NINEP),
		Networks: []limatype.Network{
			{Lima: "shared"},
		},
		SSH: limatype.SSH{
			LoadDotSSHPubKeys: new(false),
			ForwardAgent:      new(false),
		},
		PropagateProxyEnv: new(false),
		PortForwards: []limatype.PortForward{
			{
				GuestIP:           net.IPv4zero,
				GuestIPMustBeZero: new(false),
				GuestPortRange:    [2]int{1, 65535},
				Proto:             limatype.ProtoAny,
				Ignore:            true,
			},
		},
		Containerd: limatype.Containerd{
			System: new(false),
			User:   new(false),
		},
	}

	if usesVZ(architecture, host) {
		document.VMType = new(limatype.VZ)
		document.MountType = new(limatype.VIRTIOFS)
		document.Networks = []limatype.Network{
			{VZNAT: new(true)},
		}
	}

	return document
}

func managementUser(hostUID int) limatype.User {
	adminUID := uint32(1000)
	if hostUID == 1000 {
		adminUID = 1001
	}

	return limatype.User{
		Name:             new("limanix-admin"),
		Home:             new("/home/limanix-admin"),
		UID:              &adminUID,
		PasswordlessSudo: new(true),
	}
}

func templateMounts(cfg config.Config, managedHome, runtimeDir string) []limatype.Mount {
	mounts := []limatype.Mount{
		{
			Location:   managedHome,
			MountPoint: new(string(cfg.User.Home)),
			Writable:   new(true),
		},
		{
			Location:   runtimeDir,
			MountPoint: new("/mnt/limanix"),
			Writable:   new(false),
		},
	}

	for _, mount := range cfg.Mounts {
		mounts = append(mounts, limatype.Mount{
			Location:   mount.Source,
			MountPoint: new(string(mount.Target)),
			Writable:   new(mount.Mode == "rw"),
		})
	}

	// Writable, so that tools that rewrite their own settings change the files on the Mac.
	if cfg.Shell.Dotfiles != "" {
		mounts = append(mounts, limatype.Mount{
			Location:   string(cfg.Shell.Dotfiles),
			MountPoint: new(config.DotfilesTarget),
			Writable:   new(true),
		})
	}

	sort.SliceStable(mounts, func(i, j int) bool {
		return strings.Count(*mounts[i].MountPoint, "/") < strings.Count(*mounts[j].MountPoint, "/")
	})

	return mounts
}
