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
	switch {
	case len(entry.Shells) > 0:
		message = fmt.Sprintf("%s is a prompt; choose it in [shell]: prompt = %q", choice.ID, entry.Module)
	case entry.Kind == catalog.KindShell:
		message = fmt.Sprintf("%s is part of the shell; choose it in [shell]: %s", choice.ID, config.ShellSetting(entry.Module))
	default:
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

// unknownPrompt lists the prompts that the bundled catalog offers for the preset's shell.
func (r *Registry) unknownPrompt(choice config.Choice) error {
	message := fmt.Sprintf("the bundled catalog has no prompt %q; %s", choice.ID.Selector(), r.promptAdvice(choice.Shell))
	return &config.Error{Field: choice.Field, Message: message, Cause: ErrUnknownSystem}
}

// checkPrompt requires a prompt that runs in the preset's shell.
func (r *Registry) checkPrompt(choice config.Choice, entry catalog.Entry) error {
	switch {
	case len(entry.Shells) == 0:
		message := fmt.Sprintf("%s is not a prompt; %s", entry.Module, r.promptAdvice(choice.Shell))
		return &config.Error{Field: choice.Field, Message: message, Cause: ErrNotPrompt}
	case !slices.Contains(entry.Shells, choice.Shell):
		message := fmt.Sprintf("%s runs in %s, and this preset's shell is %s; %s",
			entry.Module, strings.Join(entry.Shells, " and "), choice.Shell, r.promptAdvice(choice.Shell))
		return &config.Error{Field: choice.Field, Message: message, Cause: ErrPromptShell}
	}

	return nil
}

func (r *Registry) promptAdvice(shell string) string {
	var prompts []string
	for _, entry := range r.system {
		if entry.Line == "" && slices.Contains(entry.Shells, shell) {
			prompts = append(prompts, entry.Module)
		}
	}

	if len(prompts) == 0 {
		return `set prompt = "none"`
	}

	slices.Sort(prompts)
	return "choose one of " + strings.Join(prompts, ", ") + ", or none"
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
