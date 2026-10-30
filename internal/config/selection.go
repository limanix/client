package config

import (
	"fmt"
	"slices"

	"github.com/limanix/client/internal/domain"
)

// Choice pairs a selected module with its configuration field for diagnostics.
type Choice struct {
	ID    domain.ModuleID
	Field string
	// Shell is the shell a prompt must run in, the shell of the preset; empty for other modules.
	Shell string
}

// Selection lists modules and packages added to the base guest system.
type Selection struct {
	// Modules lists the shell components, then the toolchains, then [nixos] modules in their order.
	Modules []Choice
	// Packages lists the packages of the shell preset, then [nixos] packages.
	Packages []domain.PackageName
}

var presetModules = map[string][]string{
	PresetTerminal: {"cli-tools", "zsh", "lazygit", "yazi"},
	PresetMinimal:  {"cli-tools"},
}

// presetShells names the login shell of each preset, the shell a prompt must run in.
var presetShells = map[string]string{
	PresetTerminal: "zsh",
	PresetMinimal:  "bash",
}

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
	{"java", "java", true},
	{"maven", "maven", true},
	{"gradle", "gradle", true},
	{"gcc", "gcc", true},
	{"clang", "clang", true},
}

// Selection resolves shell and toolchain choices to catalog selectors.
// The module registry checks whether each selector exists.
func (config Config) Selection() Selection {
	return Selection{
		Modules:  config.selectedModules(),
		Packages: config.selectedPackages(),
	}
}

func (config Config) selectedModules() []Choice {
	var modules []Choice
	add := func(selector, field string) {
		modules = append(modules, Choice{ID: domain.ModuleID("lmx:" + selector), Field: field})
	}

	for _, name := range presetModules[config.Shell.Preset] {
		add(name, "shell.preset")
	}
	if config.Shell.Prompt != None {
		modules = append(modules, Choice{
			ID:    domain.ModuleID("lmx:" + config.Shell.Prompt),
			Field: "shell.prompt",
			Shell: presetShells[config.Shell.Preset],
		})
	}
	if config.Shell.Editor != None {
		add(config.Shell.Editor, "shell.editor")
	}
	if config.Shell.Multiplexer != None {
		add(config.Shell.Multiplexer, "shell.multiplexer")
	}

	for _, toolchain := range toolchains {
		value := config.Toolchains.value(toolchain.key)
		if value == "" {
			continue
		}
		selector := toolchain.module
		if toolchain.versioned {
			selector += "-" + value
		}
		add(selector, "toolchains."+toolchain.key)
	}

	for index, id := range config.NixOS.Modules {
		modules = append(modules, Choice{ID: id, Field: fmt.Sprintf("nixos.modules[%d]", index)})
	}

	return modules
}

func (config Config) selectedPackages() []domain.PackageName {
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
	return append(packages, config.NixOS.Packages...)
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
	case "java":
		return string(toolchains.Java)
	case "maven":
		return string(toolchains.Maven)
	case "gradle":
		return string(toolchains.Gradle)
	case "gcc":
		return string(toolchains.Gcc)
	case "clang":
		return string(toolchains.Clang)
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
