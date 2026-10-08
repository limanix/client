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
	Name     string            `json:"name"`
	Address  string            `json:"address"`
	Disk     *domain.DiskUsage `json:"disk"`
	Home     *string           `json:"home"`
	LimaName *string           `json:"lima_name"`
	Error    *string           `json:"error"`
	Notice   string            `json:"notice,omitempty"`

	OperationStatus *domain.Status       `json:"state"`
	Arch            *domain.Architecture `json:"arch"`
	BackendStatus   *lima.Status         `json:"status"`
}
