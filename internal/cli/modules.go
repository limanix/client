package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/limanix/client/internal/domain"
	"github.com/limanix/client/internal/modules"
)

func modulesCommand(dependencies Dependencies) *cobra.Command {
	catalog := &cobra.Command{
		Use:   "modules COMMAND",
		Short: "Manage bundled and third-party NixOS modules.",

		RunE: func(_ *cobra.Command, _ []string) error {
			return usageError(errMissingModuleCommand)
		},
	}
	catalog.AddCommand(
		moduleListCommand(dependencies),
		moduleAddCommand(dependencies),
		moduleRemoveCommand(dependencies),
	)
	return catalog
}

func moduleListCommand(dependencies Dependencies) *cobra.Command {
	var asJSON, quiet bool

	command := &cobra.Command{
		Use:   "list",
		Short: "List available module identifiers.",
		Args:  exactArgs(0),

		RunE: func(cmd *cobra.Command, _ []string) error {
			if quiet && asJSON {
				return usageError(errQuietJSON)
			}

			registry, err := dependencies.Registry()
			if err != nil {
				return err
			}

			entries, err := registry.Available(cmd.Context())
			if err != nil {
				return err
			}

			switch {
			case asJSON:
				return writeJSON(cmd.OutOrStdout(), entries)
			case quiet:
				return writeNames(cmd.OutOrStdout(), entries, func(entry modules.Info) string { return entry.Name })
			}

			return writeModules(cmd.OutOrStdout(), entries)
		},
	}
	command.Flags().BoolVar(&asJSON, "json", false, "Print machine-readable JSON.")
	command.Flags().BoolVarP(&quiet, "quiet", "q", false, "Print only module selectors, one per line.")
	return command
}

func moduleAddCommand(dependencies Dependencies) *cobra.Command {
	return &cobra.Command{
		Use:   "add NAME DIRECTORY",
		Short: "Import a directory containing default.nix.",
		Args:  exactArgs(2),

		RunE: func(cmd *cobra.Command, args []string) error {
			name, err := domain.NewModuleName(args[0])
			if err != nil {
				return err
			}

			registry, err := dependencies.Registry()
			if err != nil {
				return err
			}

			if err = registry.Add(cmd.Context(), name, args[1]); err != nil {
				return err
			}

			_, err = fmt.Fprintf(cmd.OutOrStdout(), "Imported third-party:%s.\n", name)
			return err
		},
	}
}

func moduleRemoveCommand(dependencies Dependencies) *cobra.Command {
	return &cobra.Command{
		Use:   "remove NAME",
		Short: "Remove an imported module from the catalog.",
		Args:  exactArgs(1),

		RunE: func(cmd *cobra.Command, args []string) error {
			name, err := domain.NewModuleName(args[0])
			if err != nil {
				return err
			}

			registry, err := dependencies.Registry()
			if err != nil {
				return err
			}

			if err = registry.Remove(cmd.Context(), name); err != nil {
				return err
			}

			_, err = fmt.Fprintf(cmd.OutOrStdout(), "Removed third-party:%s from the catalog.\n", name)
			return err
		},
	}
}
