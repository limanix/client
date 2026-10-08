package cli

import (
	"context"
	"fmt"
	"strconv"

	"github.com/limanix/client/internal/domain"
	"github.com/limanix/client/internal/lima"
	"github.com/limanix/client/internal/vm"
	"github.com/limanix/client/internal/vmnet"
	"github.com/spf13/cobra"
)

func networkCommand(dependencies Dependencies) *cobra.Command {
	command := &cobra.Command{
		Use:   "network",
		Short: "Prepare macOS networking for QEMU guests and check connections to a VM.",
		Args:  exactArgs(0),
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}
	command.AddCommand(&cobra.Command{
		Use:   "setup",
		Short: "Install the bundled socket_vmnet helper and Lima sudoers with administrator approval.",
		Args:  exactArgs(0),
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := requireNativeNetworkHost(cmd.Context()); err != nil {
				return err
			}

			return vmnet.New(cmd.InOrStdin(), cmd.ErrOrStderr()).Ensure(cmd.Context())
		},
	}, networkCheckCommand(dependencies))

	return command
}

func networkCheckCommand(dependencies Dependencies) *cobra.Command {
	var udp, asJSON bool

	command := &cobra.Command{
		Use:   "check NAME PORT",
		Short: "Check why a guest port may be unreachable from the Mac.",
		Long: "Check the VM and its address, the guest firewall, the listener and its process, then connect to a TCP port " +
			"from the Mac; a UDP port has no connection to try. The command exits with status 1 when a check failed.",
		Args: exactArgs(2),

		RunE: func(cmd *cobra.Command, args []string) error {
			name, err := domain.NewVMName(args[0])
			if err != nil {
				return err
			}

			port, err := strconv.ParseUint(args[1], 10, 16)
			if err != nil || port == 0 {
				return usageError(fmt.Errorf("PORT must be a number from 1 to 65535, not %q", args[1]))
			}

			protocol := vm.TCP
			if udp {
				protocol = vm.UDP
			}

			manager, err := dependencies.Manager()
			if err != nil {
				return err
			}

			report, err := manager.NetworkCheck(cmd.Context(), name, uint16(port), protocol)
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
	command.Flags().BoolVar(&udp, "udp", false, "Check a UDP port instead of a TCP one.")
	command.Flags().BoolVar(&asJSON, "json", false, "Print machine-readable JSON.")
	return command
}

func networkInstallerCommand() *cobra.Command {
	return &cobra.Command{
		Use:    "vmnet-install",
		Short:  "Run the internal privileged network installer.",
		Hidden: true,
		Args:   exactArgs(0),
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := requireNativeNetworkHost(cmd.Context()); err != nil {
				return err
			}

			return vmnet.Install(cmd.Context(), cmd.InOrStdin(), cmd.ErrOrStderr())
		},
	}
}

func requireNativeNetworkHost(ctx context.Context) error {
	if err := lima.RequireMacOS(ctx); err != nil {
		return err
	}

	architecture, err := lima.HostArchitecture()
	if err != nil {
		return err
	}

	return lima.RequireNativeArchitecture(architecture)
}
