package guest

import (
	"context"
	"net/netip"
	"strings"
	"time"

	"github.com/limanix/client/internal/domain"
	"github.com/limanix/client/internal/lima"
)

const statusTimeout = 10 * time.Second

// Status is what a listing shows about a running guest. An empty field was not read.
type Status struct {
	// Address is the IPv4 address of the interface on Lima's shared network.
	Address string

	// Disk is the usage of the file system that holds the Nix store.
	Disk *domain.DiskUsage

	// Notice tells how to make the guest answer, such as an update for a guest without lmx.
	Notice string
}

type statusData struct {
	Disk       *domain.DiskUsage `json:"disk"`
	Interfaces []struct {
		MAC  string   `json:"mac"`
		IPv4 []string `json:"ipv4"`
	} `json:"interfaces"`
}

// Status asks lmx in a running guest for the address and the disk usage. A guest that does not answer within
// statusTimeout or the caller's deadline gets an empty status, and a guest without lmx gets a notice.
func (guest *Guest) Status(ctx context.Context, instance lima.Instance) Status {
	if instance.Status != lima.Running {
		return Status{}
	}

	probe, cancel := context.WithTimeout(ctx, statusTimeout)
	defer cancel()

	var data statusData
	if err := guest.lmx(probe, instance.Name, &data, "lmx", "status"); err != nil {
		if missingLMX(err) {
			return Status{Notice: "the guest has no lmx yet; run limanix update"}
		}

		return Status{}
	}

	return Status{
		Address: data.address(sharedMACs(instance.Networks)),
		Disk:    data.Disk,
	}
}

// address selects the first IPv4 address of an interface on Lima's shared network; lmx lists global addresses only.
func (data statusData) address(shared map[string]bool) string {
	for _, iface := range data.Interfaces {
		if !shared[strings.ToLower(iface.MAC)] {
			continue
		}

		for _, value := range iface.IPv4 {
			if address, err := netip.ParseAddr(value); err == nil && address.Is4() {
				return address.String()
			}
		}
	}

	return ""
}

func sharedMACs(networks []lima.Network) map[string]bool {
	result := make(map[string]bool, len(networks))

	for _, network := range networks {
		if network.Shared && network.MACAddress != "" {
			result[strings.ToLower(network.MACAddress)] = true
		}
	}

	return result
}
