package guest

import (
	"testing"

	"github.com/limanix/client/internal/domain"
)

func TestDiskErrorNamesInodesOnlyWithAnInodeTable(t *testing.T) {
	for _, test := range []struct {
		usage domain.DiskUsage
		want  string
	}{
		{
			usage: domain.DiskUsage{Bytes: 64 << 30, FreeBytes: 1 << 30},
			want:  "guest disk is full or nearly full: 1.0 GiB of 64.0 GiB free; raise resources.disk or remove guest data",
		},
		{
			usage: domain.DiskUsage{Bytes: 64 << 30, FreeBytes: 1 << 30, Inodes: 1000, FreeInodes: 5},
			want:  "guest disk is full or nearly full: 5 of 1000 inodes and 1.0 GiB of 64.0 GiB free; raise resources.disk or remove guest data",
		},
	} {
		if got := (&DiskError{Usage: test.usage}).Error(); got != test.want {
			t.Errorf("got %q, want %q", got, test.want)
		}
	}
}
