package cli

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/limanix/client/internal/domain"
)

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
	// Guest flags must reach the remote command unchanged.
	command.Flags().String("session", "", "Create or attach to a named persistent session; requires a session provider in the VM.")
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
				return "", nil, usageError(errDuplicateSession)
			}
			hasSession = true
			if arg == "--session" {
				if len(args) < 2 {
					return "", nil, usageError(errMissingSession)
				}
				session, args = args[1], args[2:]
			} else {
				session, args = strings.TrimPrefix(arg, "--session="), args[1:]
			}
			if session == "" {
				return "", nil, usageError(errEmptySession)
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
		return "", nil, usageError(errMissingVMName)
	}
	if hasSession {
		if len(args) != 0 {
			return "", nil, usageError(errSessionCommand)
		}
		args = []string{"limanix-session", session}
	}
	return name, args, nil
}
