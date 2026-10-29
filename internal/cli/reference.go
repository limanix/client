package cli

import (
	"io"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/limanix/client/internal/config"
	"github.com/limanix/client/internal/domain"
	"github.com/limanix/client/internal/vm"
)

// Reference describes the commands and the configuration of limanix; the LimaNix site reads it from reference.json.
type Reference struct {
	Commands []Entry        `json:"commands"`
	Config   []config.Field `json:"config"`
	Example  string         `json:"example"`
}

// Entry is a command: its full name, usage line, summary, description, flags, and the checks it runs on the Mac.
type Entry struct {
	Name    string  `json:"name"`
	Usage   string  `json:"usage"`
	About   string  `json:"about"`
	Details *string `json:"details"`
	Args    []Arg   `json:"args"`
	Checks  []Check `json:"checks"`
}

// Arg is a flag of a command.
type Arg struct {
	Name     string   `json:"name"`
	Short    *string  `json:"short"`
	Value    *string  `json:"value"`
	Required bool     `json:"required"`
	Default  *string  `json:"default"`
	Values   []Choice `json:"values"`
	About    string   `json:"about"`
	Details  *string  `json:"details"`
}

// Choice is a value that an argument accepts.
type Choice struct {
	Name  string `json:"name"`
	About string `json:"about"`
}

// Check is a check that a command runs on the Mac.
type Check struct {
	ID   string `json:"id"`
	What string `json:"what"`
}

// BuildReference walks the command tree and the configuration model without running a handler. The example renders
// the defaults of an arm64 Mac, so every machine builds the same reference.
func BuildReference() (Reference, error) {
	fields, err := config.Reference()
	if err != nil {
		return Reference{}, err
	}

	defaults := config.Default()
	defaults.Resources.Arch = domain.ARM64
	example, err := config.Render(defaults)
	if err != nil {
		return Reference{}, err
	}

	root := Command(IO{In: strings.NewReader(""), Out: io.Discard, Err: io.Discard}, Dependencies{})
	var commands []Entry
	walkCommands(root, &commands)

	return Reference{Commands: commands, Config: fields, Example: string(example)}, nil
}

func walkCommands(command *cobra.Command, commands *[]Entry) {
	if command.Hidden {
		return
	}

	entry := Entry{
		Name:    command.CommandPath(),
		Usage:   command.UseLine(),
		About:   command.Short,
		Details: optional(command.Long),
		Args:    []Arg{},
		Checks:  []Check{},
	}
	command.NonInheritedFlags().VisitAll(func(flag *pflag.Flag) {
		if !flag.Hidden {
			entry.Args = append(entry.Args, argument(flag))
		}
	})
	for _, id := range checksOf(entry.Name) {
		entry.Checks = append(entry.Checks, Check{ID: string(id), What: id.What()})
	}
	*commands = append(*commands, entry)

	for _, child := range command.Commands() {
		walkCommands(child, commands)
	}
}

func checksOf(name string) []vm.CheckID {
	switch name {
	case "limanix doctor":
		return vm.DoctorChecks()
	case "limanix network check":
		return vm.NetworkChecks()
	default:
		return nil
	}
}

func argument(flag *pflag.Flag) Arg {
	value, usage := pflag.UnquoteUsage(flag)
	arg := Arg{
		Name:   "--" + flag.Name,
		Value:  optional(value),
		Values: []Choice{},
		About:  usage,
	}
	if flag.Shorthand != "" {
		arg.Short = optional("-" + flag.Shorthand)
	}
	if required := flag.Annotations[cobra.BashCompOneRequiredFlag]; len(required) > 0 && required[0] == "true" {
		arg.Required = true
	}
	switch flag.DefValue {
	case "", "false", "0", "[]":
	default:
		arg.Default = optional(flag.DefValue)
	}

	return arg
}

func optional(text string) *string {
	if text == "" {
		return nil
	}

	return &text
}
