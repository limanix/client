package guest

import (
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

func gibibytes(bytes uint64) string {
	return fmt.Sprintf("%.1f GiB", float64(bytes)/(1<<30))
}
