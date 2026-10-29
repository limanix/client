package cli

import (
	"io"
	"reflect"
	"slices"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/limanix/client/internal/config"
	"github.com/limanix/client/internal/domain"
	"github.com/limanix/client/internal/modules"
	"github.com/limanix/client/internal/vm"
)

// Reference describes the commands and the configuration of limanix; the LimaNix site reads it from reference.json.
type Reference struct {
	Commands     []Entry        `json:"commands"`
	ExitStatuses []ExitStatus   `json:"exit_statuses"`
	Environment  []Variable     `json:"environment"`
	Config       []config.Field `json:"config"`
	Example      string         `json:"example"`
}

// Entry is a command: its full name, usage line, summary, description, arguments and flags, the checks it runs on the
// Mac, the lines it prints on success, and the columns and JSON fields of its result.
type Entry struct {
	Name    string   `json:"name"`
	Usage   string   `json:"usage"`
	About   string   `json:"about"`
	Details *string  `json:"details"`
	Args    []Arg    `json:"args"`
	Checks  []Item   `json:"checks"`
	Output  []string `json:"output"`
	Columns []Item   `json:"columns"`
	JSON    []Item   `json:"json"`
}

// Arg is a positional argument or a flag of a command.
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

// Item is a check, a column or a JSON field, and what it holds.
type Item struct {
	ID   string `json:"id"`
	What string `json:"what"`
}

// ExitStatus is a status limanix exits with; a null status stands for the statuses of a guest command.
type ExitStatus struct {
	Status *int   `json:"status"`
	What   string `json:"what"`
}

// Variable is an environment variable that limanix reads.
type Variable struct {
	ID      string `json:"id"`
	Default string `json:"default"`
	What    string `json:"what"`
}

var environment = []Variable{
	{
		ID:      "LIMANIX_HOME",
		Default: "~/Library/Application Support/Limanix",
		What:    "LimaNix state: VM records, prepared configurations and imported modules.",
	},
	{
		ID:      "LIMA_HOME",
		Default: "~/.lima",
		What:    "Lima's instances, each in a directory with its host agent logs `ha.stdout.log` and `ha.stderr.log`.",
	},
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

	return Reference{
		Commands:     commands,
		ExitStatuses: exitStatuses,
		Environment:  environment,
		Config:       fields,
		Example:      string(example),
	}, nil
}

func walkCommands(command *cobra.Command, commands *[]Entry) {
	if command.Hidden {
		return
	}

	name := command.CommandPath()
	entry := Entry{
		Name:    name,
		Usage:   command.UseLine(),
		About:   command.Short,
		Details: optional(command.Long),
		Args:    operands(name),
		Checks:  []Item{},
		Output:  output(name),
		Columns: columns(name),
		JSON:    jsonOutput(name),
	}
	command.NonInheritedFlags().VisitAll(func(flag *pflag.Flag) {
		if !flag.Hidden {
			entry.Args = append(entry.Args, argument(flag))
		}
	})
	for _, id := range checksOf(name) {
		entry.Checks = append(entry.Checks, Item{ID: string(id), What: id.What()})
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

func operands(name string) []Arg {
	vmName := operand("NAME", "string", true, "VM name.")

	switch name {
	case "limanix start", "limanix stop", "limanix doctor", "limanix delete":
		return []Arg{vmName}
	case "limanix shell":
		return []Arg{vmName, operand("COMMAND ...", "strings", false,
			"Guest command and its arguments, after `--`. They reach the guest unchanged; no guest shell parses them.")}
	case "limanix network check":
		return []Arg{vmName, operand("PORT", "uint16", true, "Port number from 1 to 65535.")}
	case "limanix modules add":
		return []Arg{
			operand("NAME", "string", true, "Module name without `third-party:`: up to 63 lowercase letters, digits "+
				"and single hyphens, with every hyphen-separated part starting with a letter."),
			operand("DIRECTORY", "string", true, "Directory with a regular file named `default.nix` at its root. It may "+
				"be relative, absolute or start with `~`."),
		}
	case "limanix modules remove":
		return []Arg{operand("NAME", "string", true, "Name of an imported module, without `third-party:`.")}
	case "limanix first-config":
		return []Arg{operand("PATH", "string", false,
			"Existing directory to write `limanix.toml` into; the current directory when omitted.")}
	case "limanix help":
		return []Arg{operand("COMMAND ...", "strings", false, "Command to explain, such as `network check`.")}
	default:
		return []Arg{}
	}
}

func operand(name, value string, required bool, about string) Arg {
	return Arg{Name: name, Value: optional(value), Required: required, Values: []Choice{}, About: about}
}

func output(name string) []string {
	switch name {
	case "limanix create":
		return []string{"Created NAME. Managed home: PATH"}
	case "limanix update":
		return []string{"Updated NAME."}
	case "limanix start":
		return []string{"Started NAME."}
	case "limanix stop":
		return []string{"Stopped NAME."}
	case "limanix delete":
		return []string{"Deleted NAME.", "Preserved managed home: PATH"}
	case "limanix first-config":
		return []string{"Wrote PATH."}
	case "limanix modules add":
		return []string{"Imported third-party:NAME."}
	case "limanix modules remove":
		return []string{"Removed third-party:NAME from the catalog."}
	default:
		return []string{}
	}
}

func columns(name string) []Item {
	if name == "limanix list" {
		return instanceColumns
	}

	return []Item{}
}

func jsonOutput(name string) []Item {
	switch name {
	case "limanix create", "limanix update", "limanix start", "limanix stop", "limanix list":
		return jsonFields(reflect.TypeFor[vm.Info](), "")
	case "limanix delete":
		return jsonFields(reflect.TypeFor[deletedVM](), "")
	case "limanix doctor":
		return jsonFields(reflect.TypeFor[vm.Report](), "", "port", "protocol")
	case "limanix network check":
		return jsonFields(reflect.TypeFor[vm.Report](), "")
	case "limanix modules list":
		return jsonFields(reflect.TypeFor[modules.Info](), "")
	default:
		return []Item{}
	}
}

func jsonFields(model reflect.Type, prefix string, skip ...string) []Item {
	items := []Item{}

	for index := range model.NumField() {
		field := model.Field(index)
		name, _, _ := strings.Cut(field.Tag.Get("json"), ",")
		doc := field.Tag.Get("doc")
		if doc == "" || slices.Contains(skip, name) {
			continue
		}

		items = append(items, Item{ID: prefix + name, What: doc})
		if field.Type.Kind() == reflect.Slice && field.Type.Elem().Kind() == reflect.Struct {
			items = append(items, jsonFields(field.Type.Elem(), prefix+name+"[].")...)
		}
	}

	return items
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
	if required := flag.Annotations[requiredFlag]; len(required) > 0 && required[0] == "true" {
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
