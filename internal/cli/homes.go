package cli

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/limanix/client/internal/vm"
)

func homesCommand(dependencies Dependencies) *cobra.Command {
	homes := &cobra.Command{
		Use:   "homes COMMAND",
		Short: "List and remove the managed homes that limanix delete kept.",

		RunE: func(_ *cobra.Command, _ []string) error {
			return usageError(errMissingHomeCommand)
		},
	}
	homes.AddCommand(
		homeListCommand(dependencies),
		homeRemoveCommand(dependencies),
	)
	return homes
}

func homeListCommand(dependencies Dependencies) *cobra.Command {
	var asJSON, quiet bool

	command := &cobra.Command{
		Use:   "list",
		Short: "List kept homes and the space they take.",
		Args:  exactArgs(0),

		RunE: func(cmd *cobra.Command, _ []string) error {
			if quiet && asJSON {
				return usageError(errQuietJSON)
			}

			manager, err := dependencies.Manager()
			if err != nil {
				return err
			}

			entries, err := manager.KeptHomes(cmd.Context())
			if err != nil {
				return err
			}

			switch {
			case asJSON:
				return writeJSON(cmd.OutOrStdout(), entries)
			case quiet:
				return writeNames(cmd.OutOrStdout(), entries, func(entry vm.KeptHome) string { return entry.Name })
			}

			return writeKeptHomes(cmd.OutOrStdout(), entries)
		},
	}
	command.Flags().BoolVar(&asJSON, "json", false, "Print machine-readable JSON.")
	command.Flags().BoolVarP(&quiet, "quiet", "q", false, "Print only home names, one per line.")
	return command
}

func homeRemoveCommand(dependencies Dependencies) *cobra.Command {
	var all bool

	command := &cobra.Command{
		Use:   "remove [NAME ...]",
		Short: "Remove kept homes and all their files.",
		Args:  cobra.ArbitraryArgs,

		RunE: func(cmd *cobra.Command, args []string) error {
			if all == (len(args) > 0) {
				return usageError(errHomeSelection)
			}

			manager, err := dependencies.Manager()
			if err != nil {
				return err
			}

			names := args
			if all {
				entries, err := manager.KeptHomes(cmd.Context())
				if err != nil {
					return err
				}

				for _, entry := range entries {
					names = append(names, entry.Name)
				}
			}

			var failures []error

			for _, name := range names {
				if err = manager.RemoveHome(cmd.Context(), name); err == nil {
					_, err = fmt.Fprintf(cmd.OutOrStdout(), "Removed %s.\n", name)
				}

				failures = append(failures, err)
			}

			return errors.Join(failures...)
		},
	}
	command.Flags().BoolVar(&all, "all", false, "Remove every kept home instead of the named ones.")
	return command
}
