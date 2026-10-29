package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/limanix/client/internal/domain"
	"github.com/limanix/client/internal/vm"
)

func listCommand(dependencies Dependencies) *cobra.Command {
	var asJSON, quiet bool

	command := &cobra.Command{
		Use:   "list",
		Short: "List VMs managed by Limanix.",
		Args:  exactArgs(0),

		RunE: func(cmd *cobra.Command, _ []string) error {
			if quiet && asJSON {
				return usageError(errQuietJSON)
			}

			manager, err := dependencies.Manager()
			if err != nil {
				return err
			}

			entries, err := manager.FetchAll(cmd.Context())
			if err != nil {
				return err
			}

			switch {
			case asJSON:
				return writeJSON(cmd.OutOrStdout(), entries)
			case quiet:
				return writeNames(cmd.OutOrStdout(), entries, func(entry vm.Info) string { return entry.Name })
			}

			return writeInstances(cmd.OutOrStdout(), entries)
		},
	}
	command.Flags().BoolVar(&asJSON, "json", false, "Print machine-readable JSON.")
	command.Flags().BoolVarP(&quiet, "quiet", "q", false, "Print only VM names, one per line.")
	return command
}

func deleteCommand(dependencies Dependencies) *cobra.Command {
	var asJSON, force, removeHome bool

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

			if asJSON {
				result := deletedVM{Name: string(name)}
				if !removeHome && home != "" {
					result.Home = &home
				}

				return encodeJSON(cmd.OutOrStdout(), result)
			}

			if _, err = fmt.Fprintf(cmd.OutOrStdout(), "Deleted %s.\n", name); err != nil {
				return err
			}

			if !removeHome && home != "" {
				_, err = fmt.Fprintf(cmd.OutOrStdout(), "Preserved managed home: %s\n", home)
			}

			return err
		},
	}

	command.Flags().BoolVar(&force, "force", false, "Force Lima to stop and delete the VM.")
	command.Flags().BoolVar(&removeHome, "remove-home", false, "Also remove this VM's managed host home and its contents.")
	command.Flags().BoolVar(&asJSON, "json", false, "Print machine-readable JSON.")
	return command
}
