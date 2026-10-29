package cli

import "github.com/spf13/cobra"

// requiredFlag marks a flag that a command checks itself; cobra's own annotation would replace its usage error.
const requiredFlag = "limanix_required"

func markRequired(command *cobra.Command, name string) {
	_ = command.Flags().SetAnnotation(name, requiredFlag, []string{"true"})
}

func exactArgs(count int) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if err := cobra.ExactArgs(count)(cmd, args); err != nil {
			return usageError(err)
		}

		return nil
	}
}

func requiredConfig(path *string) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if err := exactArgs(0)(cmd, args); err != nil {
			return err
		}

		if *path == "" {
			return usageError(errMissingConfig)
		}

		return nil
	}
}
