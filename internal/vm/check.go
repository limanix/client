package vm

import (
	"context"
	"errors"
	"fmt"
	"net"
	"slices"
	"strconv"
	"syscall"
	"time"

	"github.com/limanix/client/internal/domain"
	"github.com/limanix/client/internal/guest"
	"github.com/limanix/client/internal/lima"
	"github.com/limanix/client/internal/state"
)

// connectTimeout bounds the connection that a TCP network check tries from the Mac.
const connectTimeout = 3 * time.Second

// Protocol is the transport protocol of a network check.
type Protocol string

const (
	TCP Protocol = "tcp"
	UDP Protocol = "udp"
)

// Report lists the checks of a VM in order: the host's, the guest's, then for TCP the connection from the Mac. Port
// and Protocol belong to a network check.
type Report struct {
	Name     string        `json:"name"`
	Port     uint16        `json:"port,omitempty"`
	Protocol Protocol      `json:"protocol,omitempty"`
	Checks   []guest.Check `json:"checks"`
}

// Failed reports whether a check failed; a warning does not fail a report.
func (report Report) Failed() bool {
	return slices.ContainsFunc(report.Checks, func(check guest.Check) bool {
		return check.Status == guest.CheckFailed
	})
}

// Doctor checks a VM: its Lima instance and last operation, the guest's address, then the guest's own checks.
func (m *Manager) Doctor(ctx context.Context, name domain.VMName) (Report, error) {
	report := Report{Name: string(name)}
	noAddress := guest.Check{
		Check:   "address",
		Status:  guest.CheckWarning,
		Message: "No IPv4 address on the shared network was read from the guest; limanix shell works without it.",
	}

	info, answers, err := m.inspect(ctx, &report, noAddress)
	if err != nil || !answers {
		return report, err
	}

	checks, err := m.guest.Doctor(ctx, *info.LimaName)
	report.Checks = append(report.Checks, answered(report.Name, checks, err)...)
	return report, ctx.Err()
}

// NetworkCheck checks why a guest port may be unreachable from the Mac: the VM and its address, the guest's firewall,
// listener, and process, then for TCP a connection from the Mac.
func (m *Manager) NetworkCheck(ctx context.Context, name domain.VMName, port uint16, protocol Protocol) (Report, error) {
	report := Report{Name: string(name), Port: port, Protocol: protocol}
	noAddress := guest.Check{
		Check:   "address",
		Status:  guest.CheckFailed,
		Message: "No IPv4 address on the shared network was read from the guest; the Mac has nothing to connect to.",
	}

	info, answers, err := m.inspect(ctx, &report, noAddress)
	if err != nil || !answers {
		return report, err
	}

	checks, err := m.guest.NetCheck(ctx, *info.LimaName, port, protocol == UDP)
	report.Checks = append(report.Checks, answered(report.Name, checks, err)...)
	if protocol == TCP && info.Address != "" {
		report.Checks = append(report.Checks, m.connect(ctx, info.Address, port, report.Failed()))
	}

	return report, ctx.Err()
}

// inspect reads the VM as list does and adds the vm check, then for a running VM the address check, or noAddress when
// the guest reported no address. The guest answers lmx commands when the VM runs, no other command changes it, and the
// guest has lmx.
func (m *Manager) inspect(ctx context.Context, report *Report, noAddress guest.Check) (Info, bool, error) {
	entry, err := m.store.Fetch(domain.VMName(report.Name))
	if err != nil {
		return Info{}, false, err
	}

	instances, err := m.listBackend(ctx, []state.Entry{entry})
	if err != nil {
		return Info{}, false, err
	}

	info := m.instanceInfo(ctx, entry, instances)
	report.Checks = append(report.Checks, vmCheck(report.Name, info))

	if info.BackendStatus == nil || *info.BackendStatus != lima.Running || busy(info) {
		return info, false, nil
	}

	switch {
	case info.Notice != "":
		report.Checks = append(report.Checks, guest.Check{
			Check:   "address",
			Status:  guest.CheckFailed,
			Message: "The guest has no lmx yet and reports neither its address nor its checks.",
			Hint:    "Run limanix update; it installs lmx.",
		})
	case info.Address == "":
		report.Checks = append(report.Checks, noAddress)
	default:
		report.Checks = append(report.Checks, guest.Check{
			Check:   "address",
			Status:  guest.CheckOK,
			Message: fmt.Sprintf("The guest has %s on the shared network.", info.Address),
		})
	}

	return info, info.Notice == "", nil
}

// vmCheck says whether the VM runs and how its last create, update, or delete ended. A record without an error has an
// operation status.
func vmCheck(name string, info Info) guest.Check {
	check := guest.Check{Check: "vm", Status: guest.CheckWarning}

	switch {
	case busy(info):
		check.Message = fmt.Sprintf("Another limanix command is %s the VM.", *info.OperationStatus)
		check.Hint = "Wait for it to finish, then check again."
	case info.BackendStatus == nil:
		check.Status = guest.CheckFailed
		check.Message = "Lima has no instance for the VM."
		check.Hint = fmt.Sprintf("Check that LIMA_HOME is the same as when you created it; limanix delete %s removes the saved record.", name)
	case *info.BackendStatus == lima.Stopped:
		check.Status = guest.CheckFailed
		check.Message = "The VM is stopped."
		check.Hint = fmt.Sprintf("Run limanix start %s.", name)
	case *info.BackendStatus != lima.Running:
		check.Status = guest.CheckFailed
		check.Message = fmt.Sprintf("The VM is not running; Lima reports %s.", *info.BackendStatus)
		check.Hint = fmt.Sprintf("See ha.stderr.log of the Lima instance %s under ~/.lima or LIMA_HOME.", *info.LimaName)
	case info.Error != nil:
		check.Message = *info.Error
	case *info.OperationStatus == domain.Interrupted:
		check.Message = "The last create, update, or delete was interrupted."
		check.Hint = "Run it again: limanix update finishes a create or update."
	case *info.OperationStatus == domain.Ready:
		check.Status = guest.CheckOK
		check.Message = "Running; the last create or update completed."
	default:
		check.Message = "The last create, update, or delete failed."
	}

	return check
}

// busy reports whether another limanix command changes the VM now: listing turns an abandoned operation into
// interrupted.
func busy(info Info) bool {
	return info.OperationStatus != nil && info.OperationStatus.InFlight()
}

// answered is the guest's checks, or a failed guest check with the error when the guest gave no answer.
func answered(name string, checks []guest.Check, err error) []guest.Check {
	if err != nil {
		return []guest.Check{{
			Check:   "guest",
			Status:  guest.CheckFailed,
			Message: err.Error(),
			Hint:    fmt.Sprintf("Check guest access with limanix shell %s -- true.", name),
		}}
	}

	return checks
}

// connect tries a TCP connection from the Mac; afterFailure says whether a check before it failed.
func (m *Manager) connect(ctx context.Context, address string, port uint16, afterFailure bool) guest.Check {
	target := net.JoinHostPort(address, strconv.FormatUint(uint64(port), 10))

	connection, err := m.dial(ctx, "tcp", target)
	if err == nil {
		_ = connection.Close()

		return guest.Check{Check: "connect", Status: guest.CheckOK, Message: fmt.Sprintf("Connected to %s from the Mac.", target)}
	}

	hint := "The guest checks passed; check a VPN, a firewall, or the Local Network permission of your terminal app."
	if afterFailure {
		hint = "Fix the failed checks above first."
	}

	return guest.Check{
		Check:   "connect",
		Status:  guest.CheckFailed,
		Message: fmt.Sprintf("Cannot connect to %s from the Mac: %s.", target, refusal(err)),
		Hint:    hint,
	}
}

// refusal says why a connection failed, such as "connection refused", without the dialer's wording.
func refusal(err error) string {
	if errno, ok := errors.AsType[syscall.Errno](err); ok {
		return errno.Error()
	}

	if failure, ok := errors.AsType[net.Error](err); ok && failure.Timeout() {
		return fmt.Sprintf("no answer within %s", connectTimeout)
	}

	return err.Error()
}
