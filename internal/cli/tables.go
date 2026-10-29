package cli

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/limanix/client/internal/domain"
	"github.com/limanix/client/internal/modules"
	"github.com/limanix/client/internal/vm"
)

var instanceColumns = []Item{
	{ID: "NAME", What: "VM name."},
	{ID: "STATUS", What: "Lima's power state, such as `Running` or `Stopped`. `Missing` when no Lima instance matches " +
		"the saved VM, or its identity record cannot be read."},
	{ID: "STATE", What: "The last LimaNix operation: `ready`, `error` or `interrupted`; `creating`, `updating` or " +
		"`deleting` while one runs; `corrupt` when the operation record cannot be read."},
	{ID: "ADDRESS", What: "The guest's IPv4 address on the shared network, or `-` when none was read."},
	{ID: "DISK", What: "Used share of the guest disk, by bytes or, when they are scarcer, by inodes, such as " +
		"`97% inodes`. `-` when the VM is stopped or did not answer."},
}

func writeInstances(output io.Writer, entries []vm.Info) error {
	if len(entries) == 0 {
		_, err := fmt.Fprintln(output, "No VMs managed by Limanix.")
		return err
	}

	// Notes would end tabwriter's aligned block. Insert them after formatting the rows.
	var table bytes.Buffer

	header := make([]string, 0, len(instanceColumns))
	for _, column := range instanceColumns {
		header = append(header, column.ID)
	}

	writer := tabwriter.NewWriter(&table, 0, 4, 2, ' ', 0)
	if _, err := fmt.Fprintln(writer, strings.Join(header, "\t")); err != nil {
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

// Show the disk limit closest to exhaustion.
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

		if _, err := fmt.Fprintf(writer, "%s\t%s\t%s\n", entry.Name, entry.Kind, detail); err != nil {
			return err
		}
	}

	return writer.Flush()
}
