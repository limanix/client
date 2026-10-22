package domain

import (
	"path"
	"regexp"
	"strings"
	"unicode/utf8"
)

var (
	vmNamePattern         = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$`)
	usernamePattern       = regexp.MustCompile(`^[a-z_][a-z0-9_-]{0,31}$`)
	moduleNamePattern     = regexp.MustCompile(`^[a-z][a-z0-9]*(?:-[a-z][a-z0-9]*)*$`)
	moduleVersionPattern  = regexp.MustCompile(`^[0-9]+(?:\.[0-9]+)*$`)
	envNamePattern        = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	packageSegmentPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_'-]*$`)
	rustToolchainPattern  = regexp.MustCompile(
		`^(?:(?:stable|beta|nightly)(?:-[0-9]{4}-[0-9]{2}-[0-9]{2})?|[0-9]+\.[0-9]+(?:\.[0-9]+)?)$`,
	)
)

// UnstablePackages is the package source prefix of the catalog's nixos-unstable revision.
const UnstablePackages = "unstable:"

// ValidateText checks string boundaries without disclosing its content in errors.
func ValidateText(value string, allowEmpty bool) error {
	if !utf8.ValidString(value) {
		return ErrInvalidUTF8
	}

	if strings.ContainsRune(value, 0) {
		return ErrNULCharacter
	}

	if value == "" && !allowEmpty {
		return ErrEmptyValue
	}

	return nil
}

// NewVMName validates a lowercase host-local VM name.
func NewVMName(value string) (VMName, error) {
	if err := ValidateText(value, false); err != nil {
		return "", err
	}

	if !vmNamePattern.MatchString(value) {
		return "", ErrInvalidVMName
	}

	return VMName(value), nil
}

// NewUsername validates a guest username and rejects reserved accounts.
func NewUsername(value string) (Username, error) {
	if err := ValidateText(value, false); err != nil {
		return "", err
	}

	if !usernamePattern.MatchString(value) || value == "root" || value == "limanix-admin" {
		return "", ErrInvalidUsername
	}

	return Username(value), nil
}

// NewGuestPath validates and normalizes a guest path without parent traversal.
func NewGuestPath(value string) (GuestPath, error) {
	if err := ValidateText(value, false); err != nil {
		return "", err
	}

	if strings.ContainsAny(value, " \t\r\n") {
		return "", ErrGuestPathWhitespace
	}

	if !strings.HasPrefix(value, "/") || strings.HasPrefix(value, "//") {
		return "", ErrInvalidGuestPath
	}

	for _, component := range strings.Split(value, "/") {
		if component == ".." {
			return "", ErrInvalidGuestPath
		}
	}

	normalized := path.Clean(value)
	if normalized == "/" {
		return "", ErrGuestRoot
	}

	return GuestPath(normalized), nil
}

// NewModuleName validates the unqualified name of a NixOS module.
func NewModuleName(value string) (ModuleName, error) {
	if err := ValidateText(value, false); err != nil {
		return "", err
	}

	if len(value) > 63 || !moduleNamePattern.MatchString(value) {
		return "", ErrInvalidModuleName
	}

	return ModuleName(value), nil
}

// NewModuleID validates CATALOG:NAME[-VERSION] without checking catalog membership.
func NewModuleID(value string) (ModuleID, error) {
	namespace, name, qualified := strings.Cut(value, ":")
	if !qualified {
		return "", ErrInvalidModuleID
	}

	if _, err := NewModuleName(namespace); err != nil {
		return "", err
	}

	base, version := splitModuleVersion(name)
	if _, err := NewModuleName(base); err != nil {
		return "", err
	}

	if version != "" {
		if err := ValidateModuleVersion(version); err != nil {
			return "", err
		}
	}

	return ModuleID(value), nil
}

// Selector returns the local module identifier, including an optional version.
func (id ModuleID) Selector() string {
	_, name, _ := strings.Cut(string(id), ":")
	return name
}

// ValidateModuleVersion checks a numeric selector.
func ValidateModuleVersion(version string) error {
	if len(version) > 63 || !moduleVersionPattern.MatchString(version) {
		return ErrInvalidModuleVersion
	}

	return nil
}

func splitModuleVersion(name string) (string, string) {
	index := strings.LastIndexByte(name, '-')
	if index < 0 || !moduleVersionPattern.MatchString(name[index+1:]) {
		return name, ""
	}

	return name[:index], name[index+1:]
}

// Namespace returns the catalog name that qualifies the module.
func (id ModuleID) Namespace() string {
	namespace, _, _ := strings.Cut(string(id), ":")
	return namespace
}

// IsThirdParty reports whether the module comes from the imported registry.
func (id ModuleID) IsThirdParty() bool {
	return id.Namespace() == "third-party"
}

// NewPackageName validates the syntax of a nixpkgs attribute path and its optional source; the guest's evaluation
// checks that it exists.
func NewPackageName(value string) (PackageName, error) {
	if err := ValidateText(value, false); err != nil {
		return "", err
	}

	attribute := value
	if source, rest, qualified := strings.Cut(value, ":"); qualified {
		if source+":" != UnstablePackages {
			return "", ErrUnsupportedPackageSource
		}
		attribute = rest
	}

	for _, segment := range strings.Split(attribute, ".") {
		if !packageSegmentPattern.MatchString(segment) {
			return "", ErrInvalidPackageName
		}
	}

	return PackageName(value), nil
}

// Unstable reports whether the package comes from the catalog's nixos-unstable revision.
func (name PackageName) Unstable() bool {
	return strings.HasPrefix(string(name), UnstablePackages)
}

// Attribute returns the attribute path without the package source.
func (name PackageName) Attribute() string {
	return strings.TrimPrefix(string(name), UnstablePackages)
}

// NewToolchainLine validates a toolchain line syntactically; the catalog decides whether the line exists.
func NewToolchainLine(value string) (ToolchainLine, error) {
	if err := ValidateText(value, true); err != nil {
		return "", err
	}

	if value != "" && ValidateModuleVersion(value) != nil {
		return "", ErrInvalidToolchainLine
	}

	return ToolchainLine(value), nil
}

// NewRustToolchain validates a rustup toolchain name without a host triple.
func NewRustToolchain(value string) (RustToolchain, error) {
	if err := ValidateText(value, true); err != nil {
		return "", err
	}

	if value != "" && !rustToolchainPattern.MatchString(value) {
		return "", ErrInvalidRustToolchain
	}

	return RustToolchain(value), nil
}

// NewOptionalHostPath validates the text of a host path that may be empty.
func NewOptionalHostPath(value string) (OptionalHostPath, error) {
	if err := ValidateText(value, true); err != nil {
		return "", err
	}

	return OptionalHostPath(value), nil
}

// NewEnvName validates an environment variable name.
func NewEnvName(value string) (EnvName, error) {
	if err := ValidateText(value, false); err != nil {
		return "", err
	}

	if !envNamePattern.MatchString(value) {
		return "", ErrInvalidEnvName
	}

	return EnvName(value), nil
}

// NewEnvValue validates literal text supported by guest environment files.
func NewEnvValue(value string) (EnvValue, error) {
	if err := ValidateText(value, true); err != nil {
		return "", err
	}

	for _, character := range value {
		if character == 0xFEFF ||
			(character >= 0xFDD0 && character <= 0xFDEF) ||
			character&0xFFFF == 0xFFFE ||
			character&0xFFFF == 0xFFFF {
			return "", ErrInvalidEnvValue
		}
	}

	return EnvValue(value), nil
}
