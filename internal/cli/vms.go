package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/limanix/client/internal/domain"
)

func listCommand(dependencies Dependencies) *cobra.Command {
	var asJSON bool

	command := &cobra.Command{
		Use:   "list",
		Short: "List VMs managed by Limanix.",
		Args:  exactArgs(0),

		RunE: func(cmd *cobra.Command, _ []string) error {
			manager, err := dependencies.Manager()
			if err != nil {
				return err
			}

			entries, err := manager.FetchAll(cmd.Context())
			if err != nil {
				return err
			}

			if asJSON {
				return writeJSON(cmd.OutOrStdout(), entries)
			}

			return writeInstances(cmd.OutOrStdout(), entries)
		},
	}
	command.Flags().BoolVar(&asJSON, "json", false, "Print machine-readable JSON.")
	return command
}

func deleteCommand(dependencies Dependencies) *cobra.Command {
	var force, removeHome bool

	command := &cobra.Command{
		Use:   "delete NAME",
		Short: "Delete a VM; preserve its managed host home by default.",
		Args:  exactArgs(1),

		RunE: func(cmd *cobra.Command, args []string) error {
			name, err := domain.NewVMName(args[0])
			if err != nil {
				return err
			}

			manager, err := dependencies.Manager()
			if err != nil {
				return err
			}

			home, err := manager.Delete(cmd.Context(), name, force, removeHome)
			if err != nil {
				return err
			}

			if _, err = fmt.Fprintf(cmd.OutOrStdout(), "Deleted %s.\n", name); err != nil {
				return err
			}

			if !removeHome {
				_, err = fmt.Fprintf(cmd.OutOrStdout(), "Preserved managed home: %s\n", home)
			}

			return err
		},
	}

	command.Flags().BoolVar(&force, "force", false, "Force Lima to stop and delete the VM.")
	command.Flags().BoolVar(&removeHome, "remove-home", false, "Also remove this VM's managed host home and its contents.")
	return command
}

func shellCommand(dependencies Dependencies) *cobra.Command {
	command := &cobra.Command{
		Use:   "shell NAME [--session SESSION | -- COMMAND ...]",
		Short: "Connect as the configured development user.",
		Long: "Connect as the configured development user. With --session, create or attach to a named persistent session. " +
			"Select a session provider in the VM configuration. --session cannot be combined with a guest command.",
		DisableFlagParsing: true,

		Args: func(cmd *cobra.Command, args []string) error {
			if err := cobra.MinimumNArgs(1)(cmd, args); err != nil {
				return usageError(err)
			}

			return nil
		},

		RunE: func(cmd *cobra.Command, args []string) error {
			if args[0] == "--help" || args[0] == "-h" {
				return cmd.Help()
			}

			value, guestCommand, err := shellArguments(args)
			if err != nil {
				return err
			}

			name, err := domain.NewVMName(value)
			if err != nil {
				return err
			}

			manager, err := dependencies.Manager()
			if err != nil {
				return err
			}

			status, err := manager.Shell(cmd.Context(), name, guestCommand)
			if err != nil {
				return err
			}

			if status != 0 {
				return &exitError{code: status}
			}

			return nil
		},
	}
	// Flags are parsed below so guest flags retain their literal meaning.
	command.Flags().String("session", "", "Create or attach to a named persistent session; requires a session provider in the VM.")
	return command
}

func doctorCommand(dependencies Dependencies) *cobra.Command {
	var asJSON bool

	command := &cobra.Command{
		Use:   "doctor NAME",
		Short: "Check a VM, its address, and the guest owner lmx.",
		Long: "Check a VM, its address, and the guest owner lmx. Each check is ok, warning, failed, or unknown, with a hint " +
			"when there is something to do. The command exits with status 1 when a check failed.",
		Args: exactArgs(1),

		RunE: func(cmd *cobra.Command, args []string) error {
			name, err := domain.NewVMName(args[0])
			if err != nil {
				return err
			}

			manager, err := dependencies.Manager()
			if err != nil {
				return err
			}

			report, err := manager.Doctor(cmd.Context(), name)
			if err != nil {
				return err
			}

			if err := writeReport(cmd.OutOrStdout(), report, asJSON); err != nil {
				return err
			}

			if report.Failed() {
				return &exitError{code: 1}
			}

			return nil
		},
	}
	command.Flags().BoolVar(&asJSON, "json", false, "Print machine-readable JSON.")
	return command
}

func shellArguments(args []string) (string, []string, error) {
	var name, session string
	var hasName, hasSession bool

	for len(args) > 0 {
		arg := args[0]
		if arg == "--" {
			args = args[1:]
			break
		}

		if arg == "--session" || strings.HasPrefix(arg, "--session=") {
			if hasSession {
				return "", nil, usageError(fmt.Errorf("--session may only be specified once"))
			}
			hasSession = true
			if arg == "--session" {
				if len(args) < 2 {
					return "", nil, usageError(fmt.Errorf("--session requires a session name"))
				}
				session, args = args[1], args[2:]
			} else {
				session, args = strings.TrimPrefix(arg, "--session="), args[1:]
			}
			if session == "" {
				return "", nil, usageError(fmt.Errorf("--session requires a nonempty session name"))
			}
			continue
		}

		if hasName {
			break
		}
		name, args = arg, args[1:]
		hasName = true
	}

	if !hasName {
		return "", nil, usageError(fmt.Errorf("a VM name is required"))
	}
	if hasSession {
		if len(args) != 0 {
			return "", nil, usageError(fmt.Errorf("--session cannot be combined with a guest command"))
		}
		args = []string{"limanix-session", session}
	}
	return name, args, nil
}
