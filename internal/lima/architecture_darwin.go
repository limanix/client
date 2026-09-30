//go:build darwin

package lima

import (
	"runtime"

	"github.com/limanix/client/internal/domain"
	"golang.org/x/sys/unix"
)

// HostArchitecture returns the hardware architecture, including Apple Silicon when an amd64 process is translated by Rosetta.
func HostArchitecture() (domain.Architecture, error) {
	return darwinHostArchitecture(runtime.GOARCH, func() (uint32, error) {
		return unix.SysctlUint32("sysctl.proc_translated")
	})
}
