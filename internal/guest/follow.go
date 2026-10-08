package guest

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

// follow runs the apply in the generation's lmxd and prints its build as it runs. The apply belongs to lmxd: when
// ctx ends, follow cancels it in the guest and returns ctx's error once lmx confirms the stop.
func (guest *Guest) follow(ctx context.Context, name, generation string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	// The follower outlives ctx to receive the outcome of the cancel.
	session, closeSession := context.WithCancel(context.WithoutCancel(ctx))
	defer closeSession()

	var (
		events   = &applyEvents{guest: guest}
		command  = []string{"sudo", lmxPackage + "/bin/lmx", "apply", "-g", generation, "--follow", "--json"}
		finished = make(chan struct{})
		failure  error
	)

	go func() {
		defer close(finished)
		failure = guest.client.Stream(session, name, command, events)
	}()

	select {
	case <-finished:
		return events.outcome(failure)
	case <-ctx.Done():
	}

	err := guest.cancelApply(ctx, name, generation, finished)
	closeSession()
	<-finished
	events.flush()

	if err != nil && events.answer == nil {
		return fmt.Errorf("cannot confirm the guest apply stopped: %w", err)
	}

	return ctx.Err()
}

// cancelApply asks lmxd to cancel the apply, which it answers once the apply stopped, and gives the follower time
// to receive the outcome. Without an apply to cancel, the follower may be starting one, which restore stops.
func (guest *Guest) cancelApply(ctx context.Context, name, generation string, finished <-chan struct{}) error {
	cleanup, release := context.WithTimeout(context.WithoutCancel(ctx), cleanupTimeout)
	defer release()

	var answer struct {
		Cancelled bool `json:"cancelled"`
	}

	if err := guest.lmx(cleanup, name, &answer, lmxPackage+"/bin/lmx", "apply", "cancel", "-g", generation); err != nil {
		return err
	}

	if answer.Cancelled {
		select {
		case <-finished:
		case <-cleanup.Done():
		}
	}

	return nil
}

// applyEvents prints the events of a followed apply and keeps the envelope that ends them.
type applyEvents struct {
	guest   *Guest
	pending []byte
	answer  []byte
}

// event is a line of `lmx apply --follow --json`; the envelope that ends the events has a contract instead.
type event struct {
	Event     *string `json:"event"`
	Contract  *int    `json:"contract"`
	Stream    string  `json:"stream"`
	Line      string  `json:"line"`
	Truncated bool    `json:"truncated"`
	Message   string  `json:"message"`
	Skipped   uint64  `json:"skipped"`
}

// Write splits the follower's standard output into lines.
func (events *applyEvents) Write(data []byte) (int, error) {
	events.pending = append(events.pending, data...)

	for {
		end := bytes.IndexByte(events.pending, '\n')
		if end < 0 {
			return len(data), nil
		}

		events.handle(events.pending[:end])
		events.pending = events.pending[end+1:]
	}
}

// flush handles a last line without its newline.
func (events *applyEvents) flush() {
	events.handle(events.pending)
	events.pending = nil
}

// handle prints an event and keeps the envelope. A phase is not printed, and an unknown event is skipped, as the
// contract requires. A line outside the contract, such as one cut off when the session closes, is skipped too.
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
		output := events.guest.Stdout
		if item.Stream == "stderr" {
			output = events.guest.Stderr
		}

		if item.Truncated {
			item.Line += " …"
		}

		_, _ = fmt.Fprintln(output, item.Line)
	case "warning":
		events.guest.Warn("%s", item.Message)
	case "lagged":
		events.guest.Warn("%d lines of the build output were skipped", item.Skipped)
	}
}

// outcome decodes the envelope that ended the events. A build that failed on a full disk carries the disk usage.
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
