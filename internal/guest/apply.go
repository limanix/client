package guest

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/limanix/client/internal/domain"
	"github.com/limanix/client/internal/lima"
)

const (
	// lmxPackage links the lmx package of the mounted generation. The link keeps the package from the garbage
	// collector until the VM restarts and clears /run.
	lmxPackage = "/run/limanix-lmx"

	// transientUnit runs the lmxd of lmxPackage while the guest's own units are stopped.
	transientUnit = "lmx-transient.service"

	// cleanupTimeout bounds each step that runs after the caller gave up: cancelling an apply, and each half of
	// returning the guest to its own lmxd.
	cleanupTimeout = 30 * time.Second
)

// errDiskFull explains a build that ran out of room before lmx could measure the disk.
var errDiskFull = errors.New("guest disk is full; raise resources.disk or remove guest data")

// Apply builds the mounted generation with the lmxd it brings, restarts the VM into it and waits until lmxd reports
// the generation converged. A *FinalizeError means that the generation runs, but lmxd could not finalize it yet.
func (guest *Guest) Apply(ctx context.Context, name, generation string, arch domain.Architecture) error {
	if err := guest.build(ctx, name, generation, arch); err != nil {
		guest.restore(ctx, name)
		return err
	}

	if err := guest.client.Stop(ctx, name); err != nil {
		return err
	}

	if err := guest.client.Start(ctx, name); err != nil {
		return err
	}

	return guest.converge(ctx, name, generation)
}

// build replaces the guest's lmxd with the one of the mounted generation: every platform, the base image
// included, then applies the generation the same way.
func (guest *Guest) build(ctx context.Context, name, generation string, arch domain.Architecture) error {
	system, err := arch.LimaArch()
	if err != nil {
		return err
	}

	// The guest's own lmxd makes room before the generation's sources are fetched. A low disk does not block the
	// apply, and a guest without lmx is skipped.
	_ = guest.Reserve(ctx, name)

	// The pattern matches only loaded units: the socket, the service, and the daemon of an earlier attempt.
	if _, err = guest.client.Run(ctx, name, []string{"sudo", "systemctl", "stop", "lmx*"}, true); err != nil {
		return err
	}

	install := []string{
		"sudo", "nix", "build",
		"--extra-experimental-features", "nix-command flakes",
		"--no-write-lock-file", "--no-update-lock-file",
		"--out-link", lmxPackage,
		"path:/mnt/limanix/flake#packages." + system + "-linux.lmx",
	}
	if _, err = guest.client.Run(ctx, name, install, false); err != nil {
		return explainSpace(err)
	}

	// With notify, systemd-run returns once lmxd listens. lmxd stops within 20 seconds; a frozen one is killed
	// before restore gives up on the stop.
	daemon := []string{
		"sudo", "systemd-run",
		"--unit=" + transientUnit, "--service-type=notify", "--property=TimeoutStopSec=25s", "--collect", "--quiet",
		lmxPackage + "/bin/lmxd", "--transient", "--config", lmxPackage + "/etc/lmx/config.json",
	}
	if _, err = guest.client.Run(ctx, name, daemon, true); err != nil {
		return err
	}

	if err = guest.follow(ctx, name, generation); err != nil {
		return err
	}

	// An interruption as the apply ends still keeps the VM from restarting.
	return ctx.Err()
}

// explainSpace adds a hint to a failed command that ran out of room on the guest disk.
func explainSpace(err error) error {
	failure, ok := errors.AsType[*lima.CommandError](err)
	if ok && strings.Contains(strings.ToLower(failure.Detail), "no space left on device") {
		return errors.Join(err, errDiskFull)
	}

	return err
}

// converge waits until the lmxd of the restarted VM reports the generation converged. Another outcome carries the
// conditions lmxd reported last, because the message of a timeout does not name them.
func (guest *Guest) converge(ctx context.Context, name, generation string) error {
	err := guest.lmx(ctx, name, nil, "lmx", "status", "--wait", "converged", "-g", generation)

	failure, ok := errors.AsType[*Error](err)
	switch {
	case !ok:
		return err
	case failure.Code == codeFinalizeFailed:
		return &FinalizeError{Message: failure.Message}
	}

	var details struct {
		Conditions []struct {
			Message string `json:"message"`
		} `json:"conditions"`
	}

	causes := []error{failure}
	if json.Unmarshal(failure.Details, &details) == nil {
		for _, condition := range details.Conditions {
			if condition.Message != failure.Message {
				causes = append(causes, errors.New(condition.Message))
			}
		}
	}

	return errors.Join(causes...)
}

// restore stops the generation's lmxd and starts the guest's own service, which brings its socket; a guest without
// one rejects the start. Each half has its own time: a stop that takes all of it still leaves time for the start.
// It is best effort: a restart of the VM resets both.
func (guest *Guest) restore(ctx context.Context, name string) {
	for _, command := range [][]string{
		{"sudo", "systemctl", "stop", transientUnit},
		{"sudo", "systemctl", "start", "lmx.service"},
	} {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), cleanupTimeout)
		_, _ = guest.client.Run(cleanup, name, command, true)
		cancel()
	}
}
