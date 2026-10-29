package guest

import (
	"context"
	"fmt"
)

// The apply belongs to lmxd and must be cancelled in the guest.
// A confirmed successful outcome takes precedence over caller cancellation.
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
	if events.outcome(failure) == nil {
		return nil
	}

	if err != nil && events.answer == nil {
		return fmt.Errorf("cannot confirm the guest apply stopped: %w", err)
	}

	return ctx.Err()
}

// lmxd confirms cancellation after the apply stops.
// If the apply has not started yet, restore stops its transient daemon.
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
