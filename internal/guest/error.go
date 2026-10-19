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

// Error names the free space, and the free inodes on a file system with an inode table.
func (err *DiskError) Error() string {
	usage := err.Usage
	free := fmt.Sprintf("%s of %s free", gibibytes(usage.FreeBytes), gibibytes(usage.Bytes))
	if usage.Inodes > 0 {
		free = fmt.Sprintf("%d of %d inodes and %s", usage.FreeInodes, usage.Inodes, free)
	}

	return "guest disk is full or nearly full: " + free + "; raise resources.disk or remove guest data"
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
