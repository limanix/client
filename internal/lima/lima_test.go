package lima

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/limanix/client/internal/config"
	"github.com/limanix/client/internal/domain"
	"github.com/limanix/client/internal/nixos"
)

func TestTemplateBoundaries(t *testing.T) {
	cfg := config.Default()
	cfg.Resources.Arch = domain.ARM64
	cfg.User = config.User{Name: "developer", Home: "/home/developer", Sudo: false}
	cfg.Mounts = []config.Mount{{Source: "/nested", Target: "/workspace/data", Mode: "rw"}, {Source: "/project", Target: "/workspace", Mode: "rw"}, {Source: "/keys", Target: "/mnt/keys", Mode: "ro"}}
	cfg.Env = map[domain.EnvName]domain.EnvValue{"API_TOKEN": "test-secret-absent-from-template"}
	for _, hostUID := range []int{501, 1000} {
		data, err := Render(cfg, "/managed/home", "/managed/runtime", "arm64", hostUID, 42022)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), "API_TOKEN") || strings.Contains(string(data), "test-secret") {
			t.Fatal("secret environment entered Lima template")
		}
		var rendered limatype.LimaYAML
		if err := json.Unmarshal(data, &rendered); err != nil {
			t.Fatal(err)
		}
		if *rendered.VMType != limatype.VZ || *rendered.MountType != limatype.VIRTIOFS || len(rendered.Networks) != 1 || !*rendered.Networks[0].VZNAT {
			t.Fatalf("unexpected native settings: %s", data)
		}
		if int(*rendered.User.UID) == hostUID || !*rendered.User.PasswordlessSudo || *rendered.User.Name != "limanix-admin" {
			t.Fatal("management identity overlaps development identity")
		}
		if *rendered.PropagateProxyEnv || *rendered.SSH.ForwardAgent || !rendered.PortForwards[0].Ignore || *rendered.PortForwards[0].GuestIPMustBeZero {
			t.Fatal("unexpected automatic exposure")
		}
		if rendered.SSH.LocalPort == nil || *rendered.SSH.LocalPort != 42022 {
			t.Fatalf("the SSH port must stay fixed: %s", data)
		}
		if len(rendered.Mounts) != 5 {
			t.Fatalf("unexpected implicit mounts: %v", rendered.Mounts)
		}
		parent, nested := -1, -1
		for index, mount := range rendered.Mounts {
			if *mount.MountPoint == "/workspace" {
				parent = index
			}
			if *mount.MountPoint == "/workspace/data" {
				nested = index
			}
			if *mount.MountPoint == "/mnt/keys" && *mount.Writable {
				t.Fatal("readonly mount became writable")
			}
		}
		if parent < 0 || nested < parent {
			t.Fatal("parent mount must precede its child")
		}
	}
}

func TestArchitectureTemplatesUseHardwareArchitecture(t *testing.T) {
	for _, test := range []struct {
		name   string
		host   domain.Architecture
		guest  domain.Architecture
		vmType limatype.VMType
	}{
		{"Apple Silicon native", domain.ARM64, domain.ARM64, limatype.VZ},
		{"Intel native", domain.AMD64, domain.AMD64, limatype.VZ},
		{"amd64 guest on Apple Silicon", domain.ARM64, domain.AMD64, limatype.QEMU},
		{"arm64 guest on Intel", domain.AMD64, domain.ARM64, limatype.QEMU},
	} {
		t.Run(test.name, func(t *testing.T) {
			cfg := config.Default()
			cfg.Resources.Arch = test.guest
			data, err := Render(cfg, "/home", "/runtime", test.host, 501, 42022)
			if err != nil {
				t.Fatal(err)
			}
			var rendered limatype.LimaYAML
			if err := json.Unmarshal(data, &rendered); err != nil {
				t.Fatal(err)
			}
			if *rendered.VMType != test.vmType || usesVZ(test.guest, test.host) != (test.vmType == limatype.VZ) {
				t.Fatalf("selected incorrect driver: %s", data)
			}
			guestArch, err := test.guest.LimaArch()
			if err != nil || *rendered.Arch != guestArch || rendered.Images[0].Arch != guestArch {
				t.Fatalf("changed guest architecture: %s", data)
			}
			if test.vmType == limatype.QEMU && (*rendered.MountType != limatype.NINEP || rendered.Networks[0].Lima != "shared") {
				t.Fatal("foreign guest requires QEMU/shared")
			}
			image, err := nixos.BaseImage(test.guest)
			if err != nil {
				t.Fatal(err)
			}
			if rendered.Images[0].Location != image.Location || rendered.Images[0].Digest.String() != "sha256:"+image.SHA256 {
				t.Fatal("template image differs from the pinned NixOS base")
			}
		})
	}
	for _, host := range []domain.Architecture{"", "aarch64", "unknown"} {
		if _, err := Render(config.Default(), "/home", "/runtime", host, 501, 42022); err == nil {
			t.Fatalf("invalid host architecture accepted: %q", host)
		}
	}
	if usesVZ("unknown", "unknown") {
		t.Fatal("invalid architectures selected VZ")
	}
}

func TestDotfilesMountWritableOnlyWhenConfigured(t *testing.T) {
	cfg := config.Default()
	mounts := func() map[string]limatype.Mount {
		t.Helper()
		data, err := Render(cfg, "/managed/home", "/managed/runtime", "arm64", 501, 42022)
		if err != nil {
			t.Fatal(err)
		}
		var rendered limatype.LimaYAML
		if err := json.Unmarshal(data, &rendered); err != nil {
			t.Fatal(err)
		}
		result := make(map[string]limatype.Mount, len(rendered.Mounts))
		for _, mount := range rendered.Mounts {
			result[*mount.MountPoint] = mount
		}
		return result
	}

	if _, exists := mounts()[config.DotfilesTarget]; exists {
		t.Fatal("dotfiles mounted without shell.dotfiles")
	}

	cfg.Shell.Dotfiles = "/Users/dev/dotfiles"
	mount, exists := mounts()[config.DotfilesTarget]
	if !exists || mount.Location != "/Users/dev/dotfiles" || !*mount.Writable {
		t.Fatalf("dotfiles mount: %+v", mount)
	}
}

func TestTemplateRejectsInvalidSSHPorts(t *testing.T) {
	for _, port := range []int{-1, 0, 65536} {
		if _, err := Render(config.Default(), "/home", "/runtime", "arm64", 501, port); !errors.Is(err, ErrSSHPort) {
			t.Fatalf("accepted SSH port %d: %v", port, err)
		}
	}
}
