package guest

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/limanix/client/internal/domain"
	"github.com/limanix/client/internal/lima"
)

const (
	diskProbeTimeout = 10 * time.Second

	// noSpace is the guest strerror text for ENOSPC; a failed build may free its partial output before the probe.
	noSpace = "no space left on device"
)

var collectGarbage = []string{"sudo", "nix-store", "--gc", "--quiet"}

// Reserve collects unreferenced store paths when bytes or inodes fall below the collection threshold.
// It returns a DiskError when collection leaves less than the minimum; unreadable usage is not an error.
func (guest *Guest) Reserve(ctx context.Context, name string) error {
	usage, err := guest.diskUsage(ctx, name)
	if err != nil || !usage.Below(domain.DiskCollectPercent) {
		return nil
	}

	// Quiet collection prints only the freed-space summary.
	_, collect := guest.client.Run(ctx, name, collectGarbage, false)

	usage, err = guest.diskUsage(ctx, name)
	if err != nil || !usage.Below(domain.DiskMinimumPercent) {
		return nil
	}

	return errors.Join(collect, &DiskError{Usage: usage})
}

// Disk reads the store file-system usage of a running guest; a failed probe yields nil.
func (guest *Guest) Disk(ctx context.Context, instance lima.Instance) *domain.DiskUsage {
	if instance.Status != lima.Running {
		return nil
	}

	usage, err := guest.diskUsage(ctx, instance.Name)
	if err != nil {
		return nil
	}

	return &usage
}

// explainFailure adds the guest disk usage to a failed apply step when the disk explains it.
func (guest *Guest) explainFailure(ctx context.Context, name string, cause error) error {
	if ctx.Err() != nil {
		return cause
	}

	usage, err := guest.diskUsage(ctx, name)
	if err != nil {
		return cause
	}

	if !usage.Below(domain.DiskMinimumPercent) && !strings.Contains(strings.ToLower(cause.Error()), noSpace) {
		return cause
	}

	return errors.Join(cause, &DiskError{Usage: usage})
}

// diskUsage counts free blocks reserved for root because the Nix daemon writes as root.
func (guest *Guest) diskUsage(ctx context.Context, name string) (domain.DiskUsage, error) {
	probe, cancel := context.WithTimeout(ctx, diskProbeTimeout)
	defer cancel()

	command := []string{"stat", "--file-system", "--format=%S %b %f %c %d", "/nix/store"}

	output, err := guest.client.Run(probe, name, command, true)
	if err != nil {
		return domain.DiskUsage{}, err
	}

	return parseDiskUsage(output)
}

func parseDiskUsage(output string) (domain.DiskUsage, error) {
	fields := strings.Fields(output)
	if len(fields) != 5 {
		return domain.DiskUsage{}, fmt.Errorf("unexpected guest file-system usage %q", output)
	}

	values := make([]uint64, len(fields))

	for index, field := range fields {
		value, err := strconv.ParseUint(field, 10, 64)
		if err != nil {
			return domain.DiskUsage{}, fmt.Errorf("unexpected guest file-system usage %q: %w", output, err)
		}

		values[index] = value
	}

	block := values[0]

	return domain.DiskUsage{
		Bytes:      values[1] * block,
		FreeBytes:  values[2] * block,
		Inodes:     values[3],
		FreeInodes: values[4],
	}, nil
}
