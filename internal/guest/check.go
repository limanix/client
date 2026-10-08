package guest

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// checkTimeout bounds lmx doctor and lmx net check, which only read the guest.
const checkTimeout = 30 * time.Second

// CheckStatus is how a check stands. Only a failed check fails a command; unknown means that it could not be decided.
type CheckStatus string

const (
	CheckOK      CheckStatus = "ok"
	CheckWarning CheckStatus = "warning"
	CheckFailed  CheckStatus = "failed"
	CheckUnknown CheckStatus = "unknown"
)

// Check is a check record of the host contract: what was checked, how it stands, and a hint when there is something
// to do.
type Check struct {
	Check   string      `json:"check"`
	Status  CheckStatus `json:"status"`
	Message string      `json:"message"`
	Hint    string      `json:"hint,omitempty"`
}

type checksData struct {
	Checks []Check `json:"checks"`
}

// Doctor runs lmx doctor in a running guest: its configuration, lmxd, the generations, and the disk.
func (guest *Guest) Doctor(ctx context.Context, name string) ([]Check, error) {
	return guest.checks(ctx, name, "lmx", "doctor")
}

// NetCheck runs lmx net check for a TCP port, or a UDP port with udp: the guest firewall, the listener, and its
// process.
func (guest *Guest) NetCheck(ctx context.Context, name string, port uint16, udp bool) ([]Check, error) {
	command := []string{"lmx", "net", "check", strconv.FormatUint(uint64(port), 10)}
	if udp {
		command = append(command, "--udp")
	}

	return guest.checks(ctx, name, command...)
}

// checks runs an lmx command that answers with check records within checkTimeout. lmx exits with status 1 when a
// check failed, but the answer is still a success.
func (guest *Guest) checks(ctx context.Context, name string, command ...string) ([]Check, error) {
	bounded, cancel := context.WithTimeout(ctx, checkTimeout)
	defer cancel()

	var data checksData
	err := guest.lmx(bounded, name, &data, command...)
	switch {
	case errors.Is(err, context.DeadlineExceeded) && ctx.Err() == nil:
		return nil, fmt.Errorf("%s gave no answer within %s", strings.Join(command, " "), checkTimeout)
	case err != nil:
		return nil, err
	}

	return data.Checks, nil
}
