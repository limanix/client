package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/limanix/client/internal/domain"
	"github.com/limanix/client/internal/modules"
	"github.com/limanix/client/internal/vm"
)

func writeJSON[T any](writer io.Writer, entries []T) error {
	if entries == nil {
		entries = []T{}
	}

	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(entries)
}

func writeInstances(output io.Writer, entries []vm.Info) error {
	if len(entries) == 0 {
		_, err := fmt.Fprintln(output, "No VMs managed by Limanix.")
		return err
	}

	// A line without columns would end the aligned block, so the rows are aligned before the notes go under them.
	var table bytes.Buffer

	writer := tabwriter.NewWriter(&table, 0, 4, 2, ' ', 0)
	if _, err := fmt.Fprintln(writer, "NAME\tSTATUS\tSTATE\tADDRESS\tDISK"); err != nil {
		return err
	}

	for _, entry := range entries {
		var (
			backendStatus   = "Missing"
			operationStatus = "corrupt"
			address         = entry.Address
		)

		if entry.BackendStatus != nil {
			backendStatus = string(*entry.BackendStatus)
		}

		if entry.OperationStatus != nil {
			operationStatus = string(*entry.OperationStatus)
		}

		if address == "" {
			address = "-"
		}

		if _, err := fmt.Fprintf(writer, "%s\t%s\t%s\t%s\t%s\n", entry.Name, backendStatus, operationStatus, address, diskUsage(entry.Disk)); err != nil {
			return err
		}
	}

	if err := writer.Flush(); err != nil {
		return err
	}

	rows := strings.SplitAfter(table.String(), "\n")
	if _, err := io.WriteString(output, rows[0]); err != nil {
		return err
	}

	for index, entry := range entries {
		if _, err := io.WriteString(output, rows[index+1]); err != nil {
			return err
		}

		for _, note := range notes(entry) {
			if _, err := fmt.Fprintf(output, "  %s\n", note); err != nil {
				return err
			}
		}
	}

	return nil
}

// notes are the lines under a row: the error, then how to make the guest answer.
func notes(entry vm.Info) []string {
	var result []string

	if entry.Error != nil {
		result = append(result, *entry.Error)
	}

	if entry.Notice != "" {
		result = append(result, entry.Notice)
	}

	return result
}

// diskUsage shows the used share of whichever guest limit is closer to running out.
func diskUsage(usage *domain.DiskUsage) string {
	if usage == nil {
		return "-"
	}

	percent, inodes := usage.Used()
	if inodes {
		return fmt.Sprintf("%d%% inodes", percent)
	}

	return fmt.Sprintf("%d%%", percent)
}

func writeModules(output io.Writer, entries []modules.Info) error {
	writer := tabwriter.NewWriter(output, 0, 4, 2, ' ', 0)

	for _, entry := range entries {
		detail := entry.Description
		if entry.Error != nil {
			detail = "error: " + *entry.Error
		}

		if _, err := fmt.Fprintf(writer, "%s\t%s\n", entry.Name, detail); err != nil {
			return err
		}
	}

	return writer.Flush()
}
