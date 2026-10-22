package config

import (
	"fmt"
	"slices"

	"github.com/limanix/client/internal/domain"
)

// Choice is a module that the configuration chooses, with the field that chooses it.
type Choice struct {
	ID    domain.ModuleID
	Field string
}

// Selection is what the configuration installs besides the platform.
type Selection struct {
	// Modules lists the shell components, then the toolchains, then [nixos] modules in their order.
	Modules []Choice
	// Packages lists the packages of the shell preset, then [nixos] packages.
	Packages []domain.PackageName
}

// presetModules are the shell components of each preset, before its editor and multiplexer.
var presetModules = map[string][]string{
	PresetTerminal: {"cli-tools", "zsh", "lazygit", "yazi"},
	PresetMinimal:  {"cli-tools"},
}

// presetPackages are the packages that each preset installs.
var presetPackages = map[string][]domain.PackageName{
	PresetTerminal: {"gh"},
}

// toolchains maps each [toolchains] key to its catalog module, in the order the table declares them.
var toolchains = []struct {
	key, module string
	versioned   bool
}{
	{"go", "go", true},
	{"rust", "rust", false},
	{"python", "python", true},
	{"nodejs", "nodejs", true},
}

// Selection resolves the shell and the toolchains to catalog selectors without consulting the catalog; the module
// registry checks that each one exists.
func (config Config) Selection() Selection {
	var modules []Choice
	add := func(selector, field string) {
		modules = append(modules, Choice{ID: domain.ModuleID("lmx:" + selector), Field: field})
	}

	for _, name := range presetModules[config.Shell.Preset] {
		add(name, "shell.preset")
	}
	if config.Shell.Editor != None {
		add(config.Shell.Editor, "shell.editor")
	}
	if config.Shell.Multiplexer != None {
		add(config.Shell.Multiplexer, "shell.multiplexer")
	}

	for _, toolchain := range toolchains {
		value := config.Toolchains.value(toolchain.key)
		switch {
		case value == "":
		case toolchain.versioned:
			add(toolchain.module+"-"+value, "toolchains."+toolchain.key)
		default:
			add(toolchain.module, "toolchains."+toolchain.key)
		}
	}

	for index, id := range config.NixOS.Modules {
		modules = append(modules, Choice{ID: id, Field: fmt.Sprintf("nixos.modules[%d]", index)})
	}

	var packages []domain.PackageName
	for _, name := range presetPackages[config.Shell.Preset] {
		listed := slices.ContainsFunc(config.NixOS.Packages, func(other domain.PackageName) bool {
			return other.Attribute() == name.Attribute()
		})
		// A listed package of the same name, from either source, replaces the preset's.
		if !listed {
			packages = append(packages, name)
		}
	}
	packages = append(packages, config.NixOS.Packages...)

	return Selection{Modules: modules, Packages: packages}
}

func (toolchains Toolchains) value(key string) string {
	switch key {
	case "go":
		return string(toolchains.Go)
	case "rust":
		return string(toolchains.Rust)
	case "python":
		return string(toolchains.Python)
	case "nodejs":
		return string(toolchains.Nodejs)
	}

	return ""
}

// ShellSetting returns the [shell] setting that chooses a catalog shell component, for diagnostics.
func ShellSetting(module string) string {
	switch module {
	case "astronvim", "neovim":
		return fmt.Sprintf("editor = %q", module)
	case "tmux":
		return `multiplexer = "tmux"`
	case "cli-tools":
		return `preset = "terminal" or preset = "minimal"`
	default:
		return `preset = "terminal"`
	}
}

// ToolchainKey returns the [toolchains] key of a catalog toolchain module.
func ToolchainKey(module string) (key string, versioned bool) {
	for _, toolchain := range toolchains {
		if toolchain.module == module {
			return toolchain.key, toolchain.versioned
		}
	}

	return module, true
}
