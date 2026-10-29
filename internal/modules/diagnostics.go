package modules

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/limanix/client/internal/config"
	"github.com/limanix/client/internal/nixos/catalog"
)

func (r *Registry) otherKind(choice config.Choice, entry catalog.Entry) error {
	var message string
	if entry.Kind == catalog.KindShell {
		message = fmt.Sprintf("%s is part of the shell; choose it in [shell]: %s", choice.ID, config.ShellSetting(entry.Module))
	} else {
		key, versioned := config.ToolchainKey(entry.Module)
		example := `"stable"`
		if lines := r.lines(entry.Module); versioned && len(lines) > 0 {
			example = strconv.Quote(lines[len(lines)-1])
		}
		if entry.Line != "" {
			example = strconv.Quote(entry.Line)
		}
		message = fmt.Sprintf("%s is a toolchain; choose its version in [toolchains], such as %s = %s", choice.ID, key, example)
	}

	return &config.Error{Field: choice.Field, Message: message, Cause: ErrOtherKind}
}

// unknownLine lists the lines that the bundled catalog offers for a toolchain.
func (r *Registry) unknownLine(choice config.Choice, key string) error {
	module, line := choice.ID.Selector(), ""
	if index := strings.LastIndexByte(module, '-'); index >= 0 {
		module, line = module[:index], module[index+1:]
	}
	lines := r.lines(module)
	message := fmt.Sprintf("the bundled catalog has no %s line %q", key, line)
	if len(lines) > 0 {
		message += "; choose one of " + strings.Join(lines, ", ")
	}

	return &config.Error{Field: choice.Field, Message: message, Cause: ErrUnknownSystem}
}

// lines returns the declared lines of a bundled module, oldest first.
func (r *Registry) lines(module string) []string {
	var lines []string
	for _, entry := range r.system {
		if entry.Module == module && entry.Line != "" {
			lines = append(lines, entry.Line)
		}
	}

	slices.SortFunc(lines, compareLines)
	return lines
}

// compareLines orders numeric lines such as 1.9 and 1.10 by their components.
func compareLines(a, b string) int {
	left, right := strings.Split(a, "."), strings.Split(b, ".")
	for index := range min(len(left), len(right)) {
		x, _ := strconv.Atoi(left[index])
		y, _ := strconv.Atoi(right[index])
		if x != y {
			return x - y
		}
	}

	return len(left) - len(right)
}
