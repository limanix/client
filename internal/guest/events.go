package guest

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
)

// applyEvents routes build output and retains the final lmx response.
type applyEvents struct {
	guest   *Guest
	pending []byte
	answer  []byte
}

// A stream line contains an event or a final response with a contract version.
type event struct {
	Event     *string `json:"event"`
	Contract  *int    `json:"contract"`
	Line      string  `json:"line"`
	Truncated bool    `json:"truncated"`
	Message   string  `json:"message"`
	Skipped   uint64  `json:"skipped"`
}

func (events *applyEvents) Write(data []byte) (int, error) {
	written := len(data)
	for {
		end := bytes.IndexByte(data, '\n')
		if end < 0 {
			events.pending = append(events.pending, data...)
			return written, nil
		}

		line := data[:end]
		if len(events.pending) != 0 {
			line = append(events.pending, line...)
			events.pending = line[:0]
		}
		events.handle(line)
		data = data[end+1:]
	}
}

func (events *applyEvents) flush() {
	events.handle(events.pending)
	events.pending = nil
}

// Unknown events and incomplete JSON lines are ignored.
func (events *applyEvents) handle(line []byte) {
	var item event
	if json.Unmarshal(line, &item) != nil {
		return
	}

	if item.Event == nil {
		if item.Contract != nil {
			events.answer = append(events.answer[:0], line...)
		}

		return
	}

	switch *item.Event {
	case "output":
		if item.Truncated {
			item.Line += " …"
		}

		_, _ = fmt.Fprintln(events.guest.Stderr, item.Line)
	case "warning":
		events.guest.Warn("%s", item.Message)
	case "lagged":
		events.guest.Warn("%d lines of the build output were skipped", item.Skipped)
	}
}

// A build failure may include disk usage for the caller's diagnostics.
func (events *applyEvents) outcome(failure error) error {
	events.flush()

	var data struct {
		State string `json:"state"`
	}

	err := decodeAnswer(events.answer, failure, &data)
	if build, ok := errors.AsType[*Error](err); ok && build.Code == codeBuildFailed {
		if usage := diskUsage(build, "disk"); usage != nil {
			return errors.Join(build, &DiskError{Usage: *usage})
		}
	}

	if err == nil && data.State != "restart_required" {
		return fmt.Errorf("lmx apply ended in state %q instead of restart_required", data.State)
	}

	return err
}
