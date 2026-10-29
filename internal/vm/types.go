package vm

import (
	"github.com/limanix/client/internal/domain"
	"github.com/limanix/client/internal/lima"
)

// Info combines persisted ownership with live backend status for a CLI listing.
//
// OperationStatus describes Limanix's workflow; BackendStatus describes Lima's lifecycle.
// Nil ownership and status fields indicate unavailable source data.
// An empty Address means that no address was discovered, not necessarily that the VM failed.
// Disk is the guest store file-system usage of a running VM; nil means it was not read.
// Error carries a state-read diagnostic or the persisted recovery message.
// Notice tells how to make a running guest answer, such as an update for a guest without lmx.
type Info struct {
	Name     string            `json:"name" doc:"VM name."`
	Address  string            `json:"address" doc:"Guest IPv4 address on the shared network, or an empty string."`
	Disk     *domain.DiskUsage `json:"disk" doc:"bytes, free_bytes, inodes and free_inodes of the guest disk, or null when not read."`
	Home     *string           `json:"home" doc:"Path of the managed home on the Mac; null when the identity record cannot be read."`
	LimaName *string           `json:"lima_name" doc:"Name of the Lima instance; null when the identity record cannot be read."`
	Error    *string           `json:"error" doc:"Saved error of the last operation, or why the VM records cannot be read; null otherwise."`
	Notice   string            `json:"notice,omitempty" doc:"What to do to make the guest answer. Present only when there is a notice."`

	OperationStatus *domain.Status       `json:"state" doc:"As STATE; null when the operation record cannot be read."`
	Arch            *domain.Architecture `json:"arch" doc:"Guest architecture, arm64 or amd64; null when the identity record cannot be read."`
	BackendStatus   *lima.Status         `json:"status" doc:"As STATUS; null when no Lima instance matches or the identity record cannot be read."`
}
