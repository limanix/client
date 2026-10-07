package guest

import (
	"encoding/json"
	"fmt"

	"github.com/limanix/client/internal/domain"
)

// DiskError reports that the guest disk has no room for an apply step.
type DiskError struct {
	Usage domain.DiskUsage
}

// Error names both limits: ext4 can run out of inodes while bytes remain free.
func (err *DiskError) Error() string {
	usage := err.Usage

	return fmt.Sprintf(
		"guest disk is full or nearly full: %d of %d inodes and %s of %s free; raise resources.disk or remove guest data",
		usage.FreeInodes, usage.Inodes, gibibytes(usage.FreeBytes), gibibytes(usage.Bytes),
	)
}

// FinalizeError reports that the applied generation runs, but lmxd could not finalize it yet: remove the older
// generations or rewrite their boot entries. lmxd tries again later.
type FinalizeError struct {
	// Message is the explanation of lmx, with the generation and the reason.
	Message string
}

// Error returns the explanation of lmx.
func (err *FinalizeError) Error() string {
	return err.Message
}

func gibibytes(bytes uint64) string {
	return fmt.Sprintf("%.1f GiB", float64(bytes)/(1<<30))
}

// diskUsage reads the store disk usage that lmx reported under key in the details of failure.
func diskUsage(failure *Error, key string) *domain.DiskUsage {
	var (
		details map[string]json.RawMessage
		usage   *domain.DiskUsage
	)

	if json.Unmarshal(failure.Details, &details) != nil || json.Unmarshal(details[key], &usage) != nil {
		return nil
	}

	return usage
}
