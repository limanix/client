package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/limanix/client/internal/domain"
	"github.com/limanix/client/internal/forward"
)

func forwardCommand(dependencies Dependencies) *cobra.Command {
	return &cobra.Command{
		Use:   "forward NAME [PORT ...]",
		Short: "Bring VM ports to localhost on the Mac until you press Ctrl-C.",
		Long: "Bring ports of a running VM to localhost on the Mac while the command runs. Without PORT, every port that a " +
			"program in the VM listens on is forwarded while it listens, to the same port on the Mac or, when that is in use, " +
			"the next free one. PORT forwards one VM port to the same port on the Mac, and LOCAL:REMOTE to another one. " +
			"Nothing stays forwarded after Ctrl-C or when the VM stops.",

		Args: func(cmd *cobra.Command, args []string) error {
			if err := cobra.MinimumNArgs(1)(cmd, args); err != nil {
				return usageError(err)
			}

			return nil
		},

		RunE: func(cmd *cobra.Command, args []string) error {
			name, err := domain.NewVMName(args[0])
			if err != nil {
				return err
			}

			ports, err := forwardPorts(args[1:])
			if err != nil {
				return err
			}

			manager, err := dependencies.Manager()
			if err != nil {
				return err
			}

			return manager.Forward(cmd.Context(), name, ports, cmd.OutOrStdout())
		},
	}
}

func forwardPorts(args []string) ([]forward.Port, error) {
	ports := make([]forward.Port, 0, len(args))

	for _, arg := range args {
		local, remote, mapped := strings.Cut(arg, ":")
		if !mapped {
			remote = local
		}

		localPort, localErr := strconv.ParseUint(local, 10, 16)
		remotePort, remoteErr := strconv.ParseUint(remote, 10, 16)

		if localErr != nil || remoteErr != nil || localPort == 0 || remotePort == 0 {
			return nil, usageError(fmt.Errorf("PORT must be a number from 1 to 65535 or LOCAL:REMOTE, not %q", arg))
		}

		ports = append(ports, forward.Port{Local: uint16(localPort), Remote: uint16(remotePort)})
	}

	return ports, nil
}
