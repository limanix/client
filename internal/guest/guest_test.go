package guest

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/limanix/client/internal/lima"
)

type call struct {
	operation string
	args      []string
	capture   bool
}
type fakeClient struct {
	calls       []call
	output      string
	failureAt   int
	shellStatus int
	run         func(context.Context) (string, error)
}

type rebuildClient struct {
	Client
	run func(context.Context, []string) (string, error)
}

func (client *rebuildClient) Run(ctx context.Context, _ string, args []string, _ bool) (string, error) {
	return client.run(ctx, args)
}

func (client *fakeClient) Run(ctx context.Context, _ string, args []string, capture bool) (string, error) {
	client.calls = append(client.calls, call{operation: "run", args: args, capture: capture})
	if client.run != nil {
		return client.run(ctx)
	}
	if client.failureAt > 0 && len(client.calls) == client.failureAt {
		return "", errors.New("build failed")
	}
	return client.output, nil
}

func (client *fakeClient) Start(context.Context, string) error {
	client.calls = append(client.calls, call{operation: "start"})
	return nil
}

func (client *fakeClient) Stop(context.Context, string) error {
	client.calls = append(client.calls, call{operation: "stop"})
	return nil
}

func (client *fakeClient) Shell(_ context.Context, _ string, args []string) (int, error) {
	client.calls = append(client.calls, call{operation: "shell", args: args})
	return client.shellStatus, nil
}

func TestApplyInstallsEnvironmentAndRebootsOnlyAfterBuild(t *testing.T) {
	client := &fakeClient{}
	client.run = func(ctx context.Context) (string, error) {
		probe := client.calls[len(client.calls)-1].args[0] == "stat"
		if deadline, exists := ctx.Deadline(); exists != probe {
			t.Fatalf("only the disk probe may have a deadline: %v %v", deadline, client.calls)
		}
		return "", nil
	}
	guest := New(client)
	if err := guest.Apply(context.Background(), "sandbox", "developer"); err != nil {
		t.Fatal(err)
	}
	if len(client.calls) != 8 || client.calls[0].args[0] != "stat" {
		t.Fatalf("unexpected apply sequence: %v", client.calls)
	}
	for index, file := range []string{"environment", "environment.sh"} {
		if !reflect.DeepEqual(client.calls[index+2].args, []string{"sudo", "install", "-m", "0644", "/mnt/limanix/" + file, "/etc/limanix/" + file}) {
			t.Fatal("runtime ENV must be available guest-wide before rebuild")
		}
	}
	build := client.calls[4]
	if build.capture || !slices.Contains(build.args, "/run/current-system/sw/bin/nixos-rebuild") {
		t.Fatalf("rebuild must stream its output: %v", build)
	}
	if !slices.Contains(build.args, "--no-update-lock-file") || !slices.Contains(build.args, "--no-write-lock-file") {
		t.Fatalf("rebuild must use the prepared lock without resolving new inputs: %v", build.args)
	}
	if client.calls[5].operation != "stop" || client.calls[6].operation != "start" || client.calls[7].args[len(client.calls[7].args)-1] != "true" {
		t.Fatalf("unexpected rebuild/reboot order: %v", client.calls)
	}
	for _, option := range []string{"--service-type=oneshot", "--property=TimeoutStartSec=infinity", "--property=KillMode=control-group"} {
		if !slices.Contains(build.args, option) {
			t.Fatalf("missing rebuild supervision option: %s", option)
		}
	}
	client = &fakeClient{failureAt: 5}
	if err := New(client).Apply(context.Background(), "sandbox", "developer"); err == nil {
		t.Fatal("rebuild failure disappeared")
	}
	for _, call := range client.calls {
		if call.operation != "run" {
			t.Fatalf("failed build rebooted the guest: %v", client.calls)
		}
	}
}

// diskClient fails the rebuild and reports the given file-system usage.
func diskClient(rebuild error, usage string) *fakeClient {
	client := &fakeClient{}
	client.run = func(context.Context) (string, error) {
		args := client.calls[len(client.calls)-1].args
		switch {
		case slices.Contains(args, "systemd-run"):
			return "", rebuild
		case args[0] == "stat":
			return usage, nil
		default:
			return "", nil
		}
	}
	return client
}

func TestApplyFailureReportsFullGuestDisk(t *testing.T) {
	for _, test := range []struct {
		name    string
		rebuild error
		usage   string
	}{
		{name: "no-inodes", rebuild: errors.New("build failed"), usage: "4096 4000000 1500000 1000000 0\n"},
		{name: "no-bytes", rebuild: errors.New("build failed"), usage: "4096 4000000 1000 1000000 900000\n"},
		{name: "reclaimed-after-failure", rebuild: errors.New("error: creating directory: No space left on device"), usage: "4096 4000000 1500000 1000000 600000\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			client := diskClient(test.rebuild, test.usage)
			err := New(client).Apply(context.Background(), "sandbox", "developer")
			full, ok := errors.AsType[*DiskError](err)
			if !ok || !errors.Is(err, test.rebuild) {
				t.Fatalf("full guest disk was not reported with the build failure: %v", err)
			}
			if full.Usage.Inodes != 1000000 || full.Usage.Bytes != 4096*4000000 {
				t.Fatalf("wrong usage: %+v", full.Usage)
			}
			if !strings.Contains(err.Error(), "resources.disk") {
				t.Fatalf("missing remedy: %v", err)
			}
			probe := client.calls[len(client.calls)-1]
			if !probe.capture || !slices.Contains(probe.args, "/nix/store") {
				t.Fatalf("usage must be read from the store file system: %v", probe)
			}
		})
	}
}

func TestApplyFailureKeepsCauseWithoutDiskShortage(t *testing.T) {
	failure := errors.New("build failed")
	for _, usage := range []string{"4096 4000000 1500000 1000000 600000\n", "unexpected\n", "4096 4000000 1500000 0 0\n"} {
		err := New(diskClient(failure, usage)).Apply(context.Background(), "sandbox", "developer")
		if !errors.Is(err, failure) {
			t.Fatalf("build failure lost: %v", err)
		}
		if _, ok := errors.AsType[*DiskError](err); ok {
			t.Fatalf("usage %q reported a full disk: %v", usage, err)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	client := &fakeClient{}
	client.run = func(context.Context) (string, error) {
		if slices.Contains(client.calls[len(client.calls)-1].args, "systemd-run") {
			cancel()
			return "", context.Canceled
		}
		return "", nil
	}
	if err := New(client).Apply(ctx, "sandbox", "developer"); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation lost: %v", err)
	}
	if last := client.calls[len(client.calls)-1]; last.args[0] == "stat" {
		t.Fatal("canceled apply probed the guest disk")
	}
}

func TestPruneRemovesReplacedGenerationsBeforeCollectingGarbage(t *testing.T) {
	client := &fakeClient{}
	if err := New(client).Prune(context.Background(), "sandbox"); err != nil {
		t.Fatal(err)
	}
	expected := []call{
		{operation: "run", args: []string{"sudo", "nix-env", "--profile", "/nix/var/nix/profiles/system", "--delete-generations", "old"}, capture: true},
		{operation: "run", args: []string{"sudo", "/nix/var/nix/profiles/system/bin/switch-to-configuration", "boot"}, capture: true},
		{operation: "run", args: []string{"sudo", "nix-store", "--gc", "--quiet"}},
	}
	if !reflect.DeepEqual(client.calls, expected) {
		t.Fatalf("unexpected prune sequence: %v", client.calls)
	}
}

func TestPruneCollectsGarbageWhenGenerationsRemain(t *testing.T) {
	client := &fakeClient{failureAt: 1}
	err := New(client).Prune(context.Background(), "sandbox")
	if err == nil {
		t.Fatal("generation removal failure disappeared")
	}
	if len(client.calls) != 2 || !slices.Contains(client.calls[1].args, "--gc") {
		t.Fatalf("boot entries must stay unchanged and garbage must still be collected: %v", client.calls)
	}

	ctx, cancel := context.WithCancel(context.Background())
	client = &fakeClient{run: func(context.Context) (string, error) {
		cancel()
		return "", context.Canceled
	}}
	if err = New(client).Prune(ctx, "sandbox"); !errors.Is(err, context.Canceled) || len(client.calls) != 1 {
		t.Fatalf("canceled prune continued: %v %v", err, client.calls)
	}
}

func TestRebuildCancellationWaitsForGuest(t *testing.T) {
	for _, test := range []struct {
		name        string
		stopFailure bool
	}{
		{name: "late-start"},
		{name: "stop-failure", stopFailure: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			stopped := make(chan struct{})
			var release sync.Once
			var unit string
			attempts := 0
			failure := errors.New("guest stop failed")
			client := &rebuildClient{run: func(session context.Context, args []string) (string, error) {
				if args[1] == "systemd-run" {
					unit = strings.TrimPrefix(args[2], "--unit=")
					cancel()
					if err := session.Err(); err != nil {
						t.Errorf("SSH closed before guest stop: %v", err)
					}
					<-stopped
					return "", errors.New("service terminated")
				}

				if !reflect.DeepEqual(args, []string{"sudo", "systemctl", "stop", unit}) || session.Err() != nil {
					t.Errorf("invalid cleanup command or context: %v, %v", args, session.Err())
				}
				if _, bounded := session.Deadline(); !bounded {
					t.Error("cleanup has no deadline")
				}

				attempts++
				if attempts == 1 && !test.stopFailure {
					return "", errors.New("unit not loaded yet")
				}

				release.Do(func() { close(stopped) })
				if test.stopFailure {
					return "", failure
				}
				return "", nil
			}}

			err := New(client).buildGeneration(ctx, "sandbox")
			if test.stopFailure {
				if !errors.Is(err, failure) || errors.Is(err, context.Canceled) {
					t.Fatalf("stop failure hidden by cancellation: %v", err)
				}
			} else if !errors.Is(err, context.Canceled) || attempts < 2 {
				t.Fatalf("launch race escaped cleanup: attempts=%d, error=%v", attempts, err)
			}
		})
	}
}

func TestAddressMatchesSharedMAC(t *testing.T) {
	client := &fakeClient{output: `[{"address":"52:55:55:00:00:01","addr_info":[{"family":"inet","scope":"global","local":"192.168.5.15"}]},{"address":"52:55:55:AA:BB:CC","addr_info":[{"family":"inet6","scope":"global","local":"2001:db8::1"},{"family":"inet","scope":"global","local":"192.0.2.10"}]}]`}
	instance := lima.Instance{Name: "sandbox", Status: lima.Running, Networks: []lima.Network{{MACAddress: "52:55:55:aa:bb:cc", Shared: true}}}
	guest := New(client)
	if address := guest.Address(context.Background(), instance); address != "192.0.2.10" {
		t.Fatalf("wrong interface selected: %s", address)
	}
	for _, output := range []string{"not-json", "null", "[1]", `[{"address":null}]`, "[]", `[{"address":"52:55:55:aa:bb:cc","addr_info":[{"family":"inet","scope":"global","local":"invalid"}]}]`} {
		client.output = output
		if address := guest.Address(context.Background(), instance); address != "" {
			t.Fatalf("unexpected unavailable address: %s", address)
		}
	}
	client.calls = nil
	instance.Status = lima.Stopped
	if guest.Address(context.Background(), instance) != "" || len(client.calls) != 0 {
		t.Fatal("queried stopped instance")
	}
}

func TestAddressBoundsProbeWithoutCancelingCaller(t *testing.T) {
	caller, cancel := context.WithCancel(context.Background())
	defer cancel()
	var probe context.Context
	client := &fakeClient{run: func(ctx context.Context) (string, error) {
		probe = ctx
		deadline, exists := ctx.Deadline()
		if remaining := time.Until(deadline); !exists || remaining <= 0 || remaining > 5*time.Second {
			t.Fatalf("address probe has no bounded deadline: %v, %v", deadline, exists)
		}
		return "", context.DeadlineExceeded
	}}
	instance := lima.Instance{Name: "sandbox", Status: lima.Running, Networks: []lima.Network{{MACAddress: "52:55:55:aa:bb:cc", Shared: true}}}
	guest := New(client)
	if address := guest.Address(caller, instance); address != "" {
		t.Fatalf("failed probe returned an address: %q", address)
	}
	if probe == nil {
		t.Fatal("address query did not reach the client")
	}
	if probe == caller || !errors.Is(probe.Err(), context.Canceled) || caller.Err() != nil {
		t.Fatalf("probe context was not released independently: probe=%v, caller=%v", probe.Err(), caller.Err())
	}
	client.run = nil
	client.output = `[{"address":"52:55:55:aa:bb:cc","addr_info":[{"family":"inet","scope":"global","local":"192.0.2.10"}]}]`
	if address := guest.Address(caller, instance); address != "192.0.2.10" {
		t.Fatalf("failed probe prevented the next address query: %q", address)
	}
}

func TestAddressHonorsEarlierCallerDeadline(t *testing.T) {
	caller, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	callerDeadline, _ := caller.Deadline()
	client := &fakeClient{run: func(ctx context.Context) (string, error) {
		if deadline, exists := ctx.Deadline(); !exists || !deadline.Equal(callerDeadline) {
			t.Fatalf("address query extended the caller deadline: %v", deadline)
		}
		<-ctx.Done()
		return "", ctx.Err()
	}}
	instance := lima.Instance{Name: "sandbox", Status: lima.Running, Networks: []lima.Network{{MACAddress: "52:55:55:aa:bb:cc", Shared: true}}}
	if address := New(client).Address(caller, instance); address != "" || !errors.Is(caller.Err(), context.DeadlineExceeded) {
		t.Fatalf("deadline did not end address query: address=%q, caller=%v", address, caller.Err())
	}
}

func TestUserCommandPreservesArgumentsInRealShell(t *testing.T) {
	executable := func(name string) string {
		path, err := exec.LookPath(name)
		if err != nil {
			t.Fatal(err)
		}
		return path
	}
	shell, printf, pwd := executable("sh"), executable("printf"), executable("pwd")
	values := []string{"space $value 'double\" end", "", "line one\nline two", "* ; : # `printf unintended`", `\backslash\`, "unicode ✓"}
	command := userCommand("dev", append([]string{printf, `%s\000`}, values...))
	index := slices.Index(command, "-c")
	if index < 0 {
		t.Fatal("missing fixed shell script")
	}
	directory := t.TempDir()
	process := exec.Command(shell, append([]string{"-c"}, command[index+1:]...)...)
	process.Env = []string{"HOME=" + directory, "PATH=/usr/bin:/bin"}
	output, err := process.Output()
	if err != nil {
		t.Fatal(err)
	}
	actual := strings.Split(strings.TrimSuffix(string(output), "\x00"), "\x00")
	if !reflect.DeepEqual(actual, values) {
		encoded, _ := json.Marshal(actual)
		t.Fatalf("guest arguments were reinterpreted: %s", encoded)
	}
	command = userCommand("dev", []string{pwd})
	index = slices.Index(command, "-c")
	process = exec.Command(shell, append([]string{"-c"}, command[index+1:]...)...)
	process.Env = []string{"HOME=" + directory, "PATH=/usr/bin:/bin"}
	output, err = process.Output()
	if err != nil {
		t.Fatal(err)
	}
	actualDirectory, err := filepath.EvalSymlinks(strings.TrimSpace(string(output)))
	if err != nil {
		t.Fatal(err)
	}
	expectedDirectory, err := filepath.EvalSymlinks(directory)
	if err != nil {
		t.Fatal(err)
	}
	if actualDirectory != expectedDirectory {
		t.Fatalf("guest command ran outside HOME: %s", actualDirectory)
	}
	if _, err := os.Stat(filepath.Join(directory, "unintended")); !os.IsNotExist(err) {
		t.Fatal("shell substitution executed")
	}
}

func TestInteractiveLoginOnceAndStatusPreserved(t *testing.T) {
	client := &fakeClient{shellStatus: 7}
	status, err := New(client).Shell(context.Background(), "sandbox", "dev", nil)
	if err != nil || status != 7 {
		t.Fatalf("shell status changed: %d %v", status, err)
	}
	if !reflect.DeepEqual(client.calls[0].args, []string{"sudo", "--login", "--user", "dev"}) {
		t.Fatalf("interactive login must use the account shell: %v", client.calls[0].args)
	}
	count := 0
	for _, arg := range client.calls[0].args {
		if arg == "--login" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("interactive shell logged in %d times", count)
	}
}

// usageClient answers successive disk probes in order and succeeds every other command.
func usageClient(usages ...string) *fakeClient {
	client := &fakeClient{}
	client.run = func(context.Context) (string, error) {
		if client.calls[len(client.calls)-1].args[0] != "stat" {
			return "", nil
		}
		usage := usages[0]
		usages = usages[1:]
		return usage, nil
	}
	return client
}

func TestReserveCollectsGarbageOnlyWhenHeadroomIsLow(t *testing.T) {
	const (
		healthy  = "4096 4000000 2000000 1000000 500000\n"
		low      = "4096 4000000 2000000 1000000 150000\n"
		critical = "4096 4000000 2000000 1000000 50000\n"
	)
	for _, test := range []struct {
		name   string
		usages []string
		calls  int
		full   bool
	}{
		{name: "healthy", usages: []string{healthy}, calls: 1},
		{name: "recovered", usages: []string{low, healthy}, calls: 3},
		{name: "still-low", usages: []string{low, low}, calls: 3},
		{name: "critical", usages: []string{critical, critical}, calls: 3, full: true},
		{name: "unreadable", usages: []string{"unexpected\n"}, calls: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			client := usageClient(test.usages...)
			err := New(client).Reserve(context.Background(), "sandbox")
			if _, full := errors.AsType[*DiskError](err); full != test.full || (!test.full && err != nil) {
				t.Fatalf("unexpected reserve result: %v", err)
			}
			if len(client.calls) != test.calls {
				t.Fatalf("unexpected reserve sequence: %v", client.calls)
			}
			if test.calls == 3 && !reflect.DeepEqual(client.calls[1].args, []string{"sudo", "nix-store", "--gc", "--quiet"}) {
				t.Fatalf("low headroom must collect unreferenced store paths: %v", client.calls)
			}
		})
	}
}

func TestDiskProbesOnlyRunningGuests(t *testing.T) {
	client := usageClient("4096 4000000 2000000 1000000 500000\n")
	instance := lima.Instance{Name: "sandbox", Status: lima.Running}
	usage := New(client).Disk(context.Background(), instance)
	if usage == nil || usage.Inodes != 1000000 || usage.FreeBytes != 4096*2000000 {
		t.Fatalf("usage not read: %+v", usage)
	}
	client = &fakeClient{}
	instance.Status = lima.Stopped
	if usage = New(client).Disk(context.Background(), instance); usage != nil || len(client.calls) != 0 {
		t.Fatalf("queried stopped instance: %+v %v", usage, client.calls)
	}
	client = &fakeClient{run: func(ctx context.Context) (string, error) {
		if _, bounded := ctx.Deadline(); !bounded {
			t.Error("disk probe has no deadline")
		}
		return "", context.DeadlineExceeded
	}}
	instance.Status = lima.Running
	if usage = New(client).Disk(context.Background(), instance); usage != nil {
		t.Fatalf("failed probe returned usage: %+v", usage)
	}
}
