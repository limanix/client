package guest

import (
	"context"
	"errors"
)

// Reserve asks lmxd to make room in the store before an update stops the VM: lmxd collects unreferenced store paths
// when the disk runs low. It returns a *DiskError when the disk stays nearly full. Any other failure, such as a guest
// without lmx, is not an error, because the apply makes room again.
func (guest *Guest) Reserve(ctx context.Context, name string) error {
	failure, ok := errors.AsType[*Error](guest.lmx(ctx, name, nil, "lmx", "store", "reserve"))
	if !ok || failure.Code != codeDiskLow {
		return nil
	}

	if usage := diskUsage(failure, "after"); usage != nil {
		return &DiskError{Usage: *usage}
	}

	return failure
}
