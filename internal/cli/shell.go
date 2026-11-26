package cli

import (
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/limanix/client/internal/domain"
)

func shellCommand(dependencies Dependencies) *cobra.Command {
	command := &cobra.Command{
		Use:   "shell NAME [--session SESSION | [--forward-agent] [-- COMMAND ...]]",
		Short: "Connect as the configured development user.",
		Long: "Connect as the configured development user. With --session, create or attach to a named persistent session. " +
			"Select a session provider in the VM configuration. --session cannot be combined with a guest command or --forward-agent. " +
			"With --forward-agent, make the Mac's active SSH agent available for this connection after verifying the VM's SSH host key.",
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

			invocation, err := shellArguments(args)
			if err != nil {
				return err
			}

			name, err := domain.NewVMName(invocation.name)
			if err != nil {
				return err
			}

			manager, err := dependencies.Manager()
			if err != nil {
				return err
			}

			shell := manager.Shell
			if invocation.forwardAgent {
				shell = manager.ShellWithAgent
			}
			status, err := shell(cmd.Context(), name, invocation.command)
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
	command.Flags().Bool("forward-agent", false, "Forward the Mac's SSH agent for this connection. Requires a pinned VM SSH host key.")
	return command
}

type shellInvocation struct {
	name         string
	command      []string
	forwardAgent bool
}

func shellArguments(args []string) (shellInvocation, error) {
	var invocation shellInvocation
	var session string
	var hasName, hasSession, hasForwardAgent bool

	for len(args) > 0 {
		arg := args[0]
		if arg == "--" {
			args = args[1:]
			break
		}

		if flag, value, assigned := strings.Cut(arg, "="); flag == "--forward-agent" {
			if hasForwardAgent {
				return shellInvocation{}, usageError(errDuplicateAgentFlag)
			}
			if !assigned {
				value = "true"
			}
			forwardAgent, err := strconv.ParseBool(value)
			if err != nil {
				return shellInvocation{}, usageError(errInvalidAgentFlag)
			}
			hasForwardAgent = true
			invocation.forwardAgent = forwardAgent
			args = args[1:]
			continue
		}

		if arg == "--session" || strings.HasPrefix(arg, "--session=") {
			if hasSession {
				return shellInvocation{}, usageError(errDuplicateSession)
			}
			hasSession = true
			if arg == "--session" {
				if len(args) < 2 {
					return shellInvocation{}, usageError(errMissingSession)
				}
				session, args = args[1], args[2:]
			} else {
				session, args = strings.TrimPrefix(arg, "--session="), args[1:]
			}
			if session == "" {
				return shellInvocation{}, usageError(errEmptySession)
			}
			continue
		}

		if hasName {
			break
		}
		invocation.name, args = arg, args[1:]
		hasName = true
	}

	if !hasName {
		return shellInvocation{}, usageError(errMissingVMName)
	}
	if hasSession {
		if hasForwardAgent {
			return shellInvocation{}, usageError(errSessionAgent)
		}
		if len(args) != 0 {
			return shellInvocation{}, usageError(errSessionCommand)
		}
		args = []string{"limanix-session", session}
	}
	invocation.command = args
	return invocation, nil
}
