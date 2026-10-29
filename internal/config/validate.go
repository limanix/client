package config

import (
	"fmt"
	"path"
	"reflect"
	"slices"
	"strings"

	"github.com/limanix/client/internal/domain"
)

// Validate checks domain values and cross-field policy without consulting the module registry.
func Validate(config Config) error {
	if err := validateValue(reflect.ValueOf(config), ""); err != nil {
		return err
	}

	switch {
	case config.SchemaVersion != 1:
		return fieldError("schema_version", "only version 1 is supported")
	case config.Resources.CPU <= 0:
		return fieldError("resources.cpu", "must be positive")
	case path.Clean(config.Home.Root) == "/":
		return fieldError("home.root", "the host root directory is not allowed")
	case config.Shell.Preset == PresetMinimal && config.Shell.Prompt != None:
		return fieldError("shell.prompt", "a prompt needs the terminal preset, which has Zsh; the minimal preset uses Bash")
	case config.Toolchains.Maven != "" && config.Toolchains.Java == "":
		return fieldError("toolchains.maven", `needs a JDK: add java to [toolchains], such as java = "21"`)
	case config.Toolchains.Gradle != "" && config.Toolchains.Java == "":
		return fieldError("toolchains.gradle", `needs a JDK: add java to [toolchains], such as java = "21"`)
	}

	if _, set := config.Env["PATH"]; set {
		return fieldError("env.PATH", "is managed by NixOS: add commands with [nixos] packages")
	}

	if err := validateHostPaths(config); err != nil {
		return err
	}

	if err := validatePorts(config.Network.Ports.TCP, "tcp"); err != nil {
		return err
	}

	if err := validatePorts(config.Network.Ports.UDP, "udp"); err != nil {
		return err
	}

	return validateMounts(config.User.Home, config.Mounts)
}

func validateHostPaths(config Config) error {
	if strings.ContainsRune(config.Home.Root, ',') {
		return fieldError("home.root", "host paths cannot contain ','")
	}

	if strings.ContainsRune(string(config.Shell.Dotfiles), ',') {
		return fieldError("shell.dotfiles", "host paths cannot contain ','")
	}

	for index, mount := range config.Mounts {
		if strings.ContainsRune(mount.Source, ',') {
			return fieldError(fmt.Sprintf("mounts[%d].source", index), "host paths cannot contain ','")
		}
	}

	return nil
}

func validatePorts(ports []int, protocol string) error {
	for index, port := range ports {
		if port < 1 || port > 65535 {
			field := fmt.Sprintf("network.ports.%s[%d]", protocol, index)

			return fieldError(field, "port must be between 1 and 65535")
		}
	}

	return nil
}

func validateMounts(home domain.GuestPath, mounts []Mount) error {
	userHome, err := checkedGuestTarget(home, "user.home")
	if err != nil {
		return err
	}

	targets := make([]string, 0, len(mounts))

	for index, mount := range mounts {
		field := fmt.Sprintf("mounts[%d].target", index)

		target, err := checkedGuestTarget(mount.Target, field)
		if err != nil {
			return err
		}

		if isWithin(userHome, target) {
			return fieldError(field, "would hide the managed user home")
		}

		for _, previous := range targets {
			if isWithin(target, previous) || isWithin(previous, target) {
				return fieldError(field, "overlaps another explicit mount")
			}
		}

		targets = append(targets, target)
	}

	return nil
}

func checkedGuestTarget(value domain.GuestPath, field string) (string, error) {
	normalized, err := domain.NewGuestPath(string(value))
	if err != nil {
		return "", wrapField(field, err.Error(), err)
	}
	if strings.ContainsRune(string(normalized), ':') {
		return "", fieldError(field, "guest mount paths cannot contain ':'")
	}

	var (
		target   = string(normalized)
		reserved = []string{
			"/etc",
			"/boot",
			"/usr",
			"/var",
			"/nix",
			"/run",
			"/dev",
			"/proc",
			"/sys",
			"/bin",
			"/sbin",
			"/mnt/limanix",
			DotfilesTarget,
			"/home/limanix-admin",
		}
	)

	for _, root := range reserved {
		if isWithin(target, root) || isWithin(root, target) {
			return "", fieldError(field, "destination is reserved for the guest system")
		}
	}

	return target, nil
}

func isWithin(candidate, root string) bool {
	return candidate == root || strings.HasPrefix(candidate, root+"/")
}

func validateString(value reflect.Value) error {
	text := value.String()

	switch value.Interface().(type) {
	case domain.VMName:
		_, err := domain.NewVMName(text)
		return err
	case domain.Username:
		_, err := domain.NewUsername(text)
		return err
	case domain.GuestPath:
		_, err := domain.NewGuestPath(text)
		return err
	case domain.ModuleID:
		_, err := domain.NewModuleID(text)
		return err
	case domain.PackageName:
		_, err := domain.NewPackageName(text)
		return err
	case domain.ToolchainLine:
		_, err := domain.NewToolchainLine(text)
		return err
	case domain.RustToolchain:
		_, err := domain.NewRustToolchain(text)
		return err
	case domain.OptionalHostPath:
		_, err := domain.NewOptionalHostPath(text)
		return err
	case domain.EnvName:
		_, err := domain.NewEnvName(text)
		return err
	case domain.EnvValue:
		_, err := domain.NewEnvValue(text)
		return err
	case domain.Architecture:
		_, err := domain.NewArchitecture(text)
		return err
	default:
		return domain.ValidateText(text, false)
	}
}

func validateValue(value reflect.Value, field string) error {
	if value.Type() == byteSizeType {
		_, err := value.Interface().(domain.ByteSize).GiB()
		if err != nil {
			return wrapField(field, err.Error(), err)
		}

		return nil
	}

	switch value.Kind() {
	case reflect.Struct:
		for index := range value.NumField() {
			var (
				item  = value.Type().Field(index)
				child = joinField(field, item.Tag.Get("toml"))
			)

			if choices := item.Tag.Get("choices"); choices != "" {
				if !slices.Contains(strings.Split(choices, ","), value.Field(index).String()) {
					return fieldError(child, "expected one of "+strings.ReplaceAll(choices, ",", ", "))
				}
			}

			if err := validateValue(value.Field(index), child); err != nil {
				return err
			}
		}
	case reflect.String:
		if err := validateString(value); err != nil {
			return wrapField(field, err.Error(), err)
		}
	case reflect.Slice:
		for index := range value.Len() {
			if err := validateValue(value.Index(index), fmt.Sprintf("%s[%d]", field, index)); err != nil {
				return err
			}
		}
	case reflect.Map:
		for _, key := range sortedMapKeys(value) {
			if err := validateValue(key, field); err != nil {
				return err
			}

			if err := validateValue(value.MapIndex(key), joinField(field, key.String())); err != nil {
				return err
			}
		}
	}

	return nil
}
