package domain

// Guest disk policy, shared by the client and the guest platform through runtime.json: below
// DiskCollectPercent free bytes or inodes, unreferenced store paths are collected; below
// DiskMinimumPercent, LimaNix warns that builds may fail.
const (
	DiskCollectPercent uint64 = 20
	DiskMinimumPercent uint64 = 10
)

// DiskUsage describes the guest file system that holds the Nix store.
//
// ext4 fixes its inode count with the file-system size, so either limit can run out first.
type DiskUsage struct {
	Bytes      uint64 `json:"bytes"`
	FreeBytes  uint64 `json:"free_bytes"`
	Inodes     uint64 `json:"inodes"`
	FreeInodes uint64 `json:"free_inodes"`
}

// Below reports whether bytes or inodes have less than percent free.
// A file system that reports no inode table, such as btrfs, has no inode limit.
func (usage DiskUsage) Below(percent uint64) bool {
	return below(usage.FreeBytes, usage.Bytes, percent) || below(usage.FreeInodes, usage.Inodes, percent)
}

// Used returns the used percentage of the scarcer limit and whether that limit is inodes.
func (usage DiskUsage) Used() (percent uint64, inodes bool) {
	bytesUsed, inodesUsed := used(usage.FreeBytes, usage.Bytes), used(usage.FreeInodes, usage.Inodes)
	if inodesUsed > bytesUsed {
		return inodesUsed, true
	}

	return bytesUsed, false
}

func below(free, total, percent uint64) bool {
	return total > 0 && free*100 < total*percent
}

func used(free, total uint64) uint64 {
	if total == 0 || free > total {
		return 0
	}

	return (total - free) * 100 / total
}
