package cli

import (
	"fmt"
	"io"
	"strings"
	"unicode"

	"github.com/spf13/cobra"

	"github.com/limanix/client/internal/domain"
	"github.com/limanix/client/internal/vm"
)

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

func writeReport(output io.Writer, report vm.Report, asJSON bool) error {
	if asJSON {
		return encodeJSON(output, report)
	}

	for _, check := range report.Checks {
		if _, err := fmt.Fprintf(output, "%-8s %-11s %s\n", check.Status, check.Check, column(check.Message)); err != nil {
			return err
		}

		if check.Hint == "" {
			continue
		}

		if _, err := fmt.Fprintf(output, "%s%s\n", checkIndent, column(check.Hint)); err != nil {
			return err
		}
	}

	return nil
}

var checkIndent = strings.Repeat(" ", 21)

// Guest messages may contain control characters and multiline SSH errors.
func column(text string) string {
	text = strings.Map(func(character rune) rune {
		if character != '\n' && unicode.IsControl(character) {
			return -1
		}

		return character
	}, strings.TrimSpace(text))

	return strings.ReplaceAll(text, "\n", "\n"+checkIndent)
}
