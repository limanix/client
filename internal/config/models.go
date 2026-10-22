package config

import "github.com/limanix/client/internal/domain"

// Resources specifies the guest architecture and compute capacity.
type Resources struct {
	Arch domain.Architecture `toml:"arch" json:"arch" doc:"Guest architecture."`
	Disk domain.ByteSize     `toml:"disk" json:"disk" doc:"Guest system disk size in GiB."`
	Mem  domain.ByteSize     `toml:"mem" json:"mem" doc:"Guest memory size in GiB."`
	CPU  int                 `toml:"cpu" json:"cpu" doc:"Guest CPU count, a positive integer."`
}

// User configures the regular account created inside the guest.
type User struct {
	Name domain.Username  `toml:"name" json:"name" doc:"Regular guest username."`
	Home domain.GuestPath `toml:"home" json:"home" doc:"Guest user's home directory."`
	Sudo bool             `toml:"sudo" json:"sudo" doc:"Passwordless sudo inside the guest."`
}

// Home selects the host directory that owns persistent guest home directories.
type Home struct {
	Root string `toml:"root" json:"root" doc:"Host root for <root>/<name>-<id>. Limanix creates this directory on the Mac and mounts it at user.home with read-write access."`
}

// Shell chooses the interactive environment of the development account from the catalog's shell components.
type Shell struct {
	Preset      string                  `toml:"preset" json:"preset" doc:"Shell preset: terminal is Zsh with Oh My Zsh, the CLI tools, Lazygit, Yazi and the GitHub CLI; minimal is Bash with the CLI tools." choices:"terminal,minimal"`
	Prompt      string                  `toml:"prompt" json:"prompt" doc:"Zsh prompt of the terminal preset; none leaves the prompt to your startup files. The minimal preset has no prompt to choose: there it defaults to and must be none." choices:"pure,starship,none"`
	Editor      string                  `toml:"editor" json:"editor" doc:"Editor: astronvim is Neovim configured with language servers for the chosen toolchains, neovim is plain Neovim. Defaults to none with the minimal preset." choices:"astronvim,neovim,none"`
	Multiplexer string                  `toml:"multiplexer" json:"multiplexer" doc:"Terminal multiplexer, which also provides the sessions of limanix shell --session. Defaults to none with the minimal preset." choices:"tmux,none"`
	Dotfiles    domain.OptionalHostPath `toml:"dotfiles" json:"dotfiles" doc:"Host directory of your dotfiles, mounted read-write at /mnt/dotfiles: each new shell links its top-level dot-entries and the entries of its .config into the home, leaving existing files alone. Empty mounts none." example:"~/dotfiles"`
}

// Toolchains chooses one version of each language toolchain from the catalog; an empty value installs none.
type Toolchains struct {
	Go     domain.ToolchainLine `toml:"go" json:"go" doc:"Go line, with gopls, Delve and GCC. Empty installs no Go." example:"1.27"`
	Rust   domain.RustToolchain `toml:"rust" json:"rust" doc:"Default toolchain of rustup, which installs it at the first start after a change: stable, beta, nightly, a dated channel such as nightly-2026-10-01, or a Rust version such as 1.90.0. Empty installs no Rust." example:"stable"`
	Python domain.ToolchainLine `toml:"python" json:"python" doc:"Python line, with virtualenv and Pyright. Empty installs no Python." example:"3.14"`
	Nodejs domain.ToolchainLine `toml:"nodejs" json:"nodejs" doc:"Node.js line, with npm, npx and the TypeScript language server. Empty installs no Node.js." example:"26"`
}

// NixOS selects trusted modules included in the generated guest configuration.
type NixOS struct {
	Modules  []domain.ModuleID    `toml:"modules" json:"modules" doc:"Selected modules: lmx:NAME or lmx:NAME-VERSION from the embedded catalog's modules, or third-party:NAME imported with limanix modules add. The [shell] and [toolchains] tables choose the other catalog modules. Omitting a version selects the module default. Multiple versions and repeated identifiers are accepted; an empty list installs no optional modules."`
	Packages []domain.PackageName `toml:"packages" json:"packages" doc:"Commands from the guest's nixpkgs by attribute path, such as jq or python3Packages.ipython; unstable:NAME takes the package from the catalog's nixos-unstable revision. Naming a package allows its license. A command that another package of the guest also provides fails the build."`
}

// Theme selects the colors of the guest.
type Theme struct {
	Flavor string `toml:"flavor" json:"flavor" doc:"Colors of lmx and the catalog modules: system uses the terminal's own colors and background, so the guest follows the theme of your terminal; latte is a light Catppuccin flavor, and frappe, macchiato and mocha are dark ones." choices:"system,latte,frappe,macchiato,mocha"`
}

// Ports declares inbound guest firewall rules by transport protocol.
type Ports struct {
	TCP []int `toml:"tcp" json:"tcp" doc:"Inbound TCP ports in the guest firewall. Services listen on a guest network interface and are reached at <guest-ip>:<port> from the Mac."`
	UDP []int `toml:"udp" json:"udp" doc:"Inbound UDP ports in the guest firewall."`
}

// Network configures connectivity and the guest firewall.
type Network struct {
	Mode  string `toml:"mode" json:"mode" doc:"Shared/NAT network. The Mac reaches the guest by its own IP address." choices:"shared"`
	Ports Ports  `toml:"ports" json:"ports" doc:"Inbound guest firewall ports."`
}

// Mount maps a host directory to a checked guest destination.
type Mount struct {
	Mode   string           `toml:"mode" json:"mode" doc:"Mount access: rw or ro." choices:"rw,ro" default:"rw"`
	Source string           `toml:"source" json:"source" doc:"Host directory to mount inside the guest." required:"true"`
	Target domain.GuestPath `toml:"target" json:"target" doc:"Mount destination inside the guest." required:"true"`
}

// Shell presets.
const (
	PresetTerminal = "terminal"
	PresetMinimal  = "minimal"
)

// None switches a shell component off.
const None = "none"

// DotfilesTarget is where the guest mounts shell.dotfiles.
const DotfilesTarget = "/mnt/dotfiles"

// Config is the public TOML contract.
type Config struct {
	SchemaVersion int                                `toml:"schema_version" json:"schema_version" doc:"Contract version, independent of the installed Limanix package version."`
	Name          domain.VMName                      `toml:"name" json:"name" doc:"Sandbox name."`
	User          User                               `toml:"user" json:"user" doc:"Regular guest user and sudo access."`
	Resources     Resources                          `toml:"resources" json:"resources" doc:"Guest architecture and compute resources."`
	Home          Home                               `toml:"home" json:"home" doc:"Host storage for the guest user's home directory."`
	Shell         Shell                              `toml:"shell" json:"shell" doc:"Shell, prompt, editor and multiplexer of the development account."`
	Toolchains    Toolchains                         `toml:"toolchains" json:"toolchains" doc:"Language toolchains, one version each."`
	NixOS         NixOS                              `toml:"nixos" json:"nixos" doc:"Trusted modules that configure the guest system."`
	Theme         Theme                              `toml:"theme" json:"theme" doc:"Colors of the guest."`
	Network       Network                            `toml:"network" json:"network" doc:"Guest network and inbound firewall ports."`
	Env           map[domain.EnvName]domain.EnvValue `toml:"env" json:"env" doc:"Guest-wide environment for login sessions and system/user services."`
	Mounts        []Mount                            `toml:"mounts" json:"mounts" doc:"Host directories mounted inside the guest."`
}

// Default returns independent collections for a new configuration.
func Default() Config {
	return Config{
		SchemaVersion: 1,
		Name:          "example-box",
		User: User{
			Name: "dev",
			Home: "/home/dev",
			Sudo: true,
		},
		Resources: Resources{
			Arch: domain.ARM64,
			Disk: domain.ByteSize(10 * domain.GiB),
			CPU:  4,
			Mem:  domain.ByteSize(8 * domain.GiB),
		},
		Home: Home{
			Root: "~/.limanix",
		},
		Shell: Shell{
			Preset:      PresetTerminal,
			Prompt:      "pure",
			Editor:      "astronvim",
			Multiplexer: "tmux",
		},
		NixOS: NixOS{
			Modules:  []domain.ModuleID{},
			Packages: []domain.PackageName{},
		},
		Theme: Theme{
			Flavor: "system",
		},
		Network: Network{
			Mode: "shared",
			Ports: Ports{
				TCP: []int{8080},
				UDP: []int{},
			},
		},
		Env: map[domain.EnvName]domain.EnvValue{
			"APP_ENV":       "development",
			"APP_LOG_LEVEL": "debug",
		},
		Mounts: []Mount{
			{
				Mode:   "rw",
				Source: "~/projects/my-project",
				Target: "/workspace",
			},
			{
				Mode:   "ro",
				Source: "~/.ssh/limanix",
				Target: "/mnt/git-keys",
			},
		},
	}
}
