package guest

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/limanix/client/internal/domain"
	"github.com/limanix/client/internal/lima"
)

const generation = "0123456789ab"

// fakeClient records guest commands joined with spaces and answers them through run and stream.
type fakeClient struct {
	mu          sync.Mutex
	calls       []string
	shellStatus int
	run         func(ctx context.Context, command string) (string, error)
	stream      func(ctx context.Context, output io.Writer) error
}

func (client *fakeClient) record(call string) {
	client.mu.Lock()
	defer client.mu.Unlock()
	client.calls = append(client.calls, call)
}

func (client *fakeClient) Run(ctx context.Context, _ string, args []string, _ bool) (string, error) {
	command := strings.Join(args, " ")
	client.record(command)
	if client.run == nil {
		return "", nil
	}
	return client.run(ctx, command)
}

func (client *fakeClient) Stream(ctx context.Context, _ string, args []string, output io.Writer) error {
	client.record(strings.Join(args, " "))
	if client.stream == nil {
		return nil
	}
	return client.stream(ctx, output)
}

func (client *fakeClient) Start(context.Context, string) error {
	client.record("start")
	return nil
}

func (client *fakeClient) Stop(context.Context, string) error {
	client.record("stop")
	return nil
}

func (client *fakeClient) Shell(_ context.Context, _ string, args []string) (int, error) {
	client.record(strings.Join(args, " "))
	return client.shellStatus, nil
}

// example reads a copy of a contract example of the pinned lmx release.
func example(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// line is an example as lmx writes it: one line.
func line(t *testing.T, name string) string {
	t.Helper()
	var compact bytes.Buffer
	if err := json.Compact(&compact, []byte(example(t, name))); err != nil {
		t.Fatal(err)
	}
	return compact.String() + "\n"
}

// commandFailure is the error of a guest command that exited with status after writing detail to standard error.
func commandFailure(t *testing.T, status int, detail string) error {
	t.Helper()
	exit, ok := errors.AsType[*exec.ExitError](exec.Command("sh", "-c", fmt.Sprintf("exit %d", status)).Run())
	if !ok {
		t.Fatalf("no exit status %d", status)
	}
	return &lima.Error{Operation: "SSH", Err: &lima.CommandError{Exit: exit, Detail: detail}}
}

// applyClient follows an apply with follow, in pieces as a pipe passes it, and answers the wait after the restart
// with wait.
func applyClient(follow, wait string) *fakeClient {
	return &fakeClient{
		run: func(_ context.Context, command string) (string, error) {
			if !strings.Contains(command, "--wait converged") {
				return "", nil
			}
			return wait, nil
		},
		stream: func(_ context.Context, output io.Writer) error {
			for chunk := range slices.Chunk([]byte(follow), 7) {
				if _, err := output.Write(chunk); err != nil {
					return err
				}
			}
			return nil
		},
	}
}

func TestApplyBuildsWithTheGenerationsLMXAndWaitsAfterRestart(t *testing.T) {
	client := applyClient(strings.TrimSuffix(example(t, "apply-follow.jsonl"), "\n"), example(t, "status.json"))
	var (
		stdout, stderr bytes.Buffer
		warnings       []string
		guest          = New(client)
	)
	guest.Stdout, guest.Stderr = &stdout, &stderr
	guest.Warn = func(format string, args ...any) { warnings = append(warnings, fmt.Sprintf(format, args...)) }

	if err := guest.Apply(context.Background(), "sandbox", generation, domain.ARM64); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"sudo lmx store reserve --json",
		"sudo systemctl stop lmx*",
		"sudo nix build --extra-experimental-features nix-command flakes --no-write-lock-file --no-update-lock-file --out-link /run/limanix-lmx path:/mnt/limanix/flake#packages.aarch64-linux.lmx",
		"sudo systemd-run --unit=lmx-transient.service --service-type=notify --property=TimeoutStopSec=25s --collect --quiet /run/limanix-lmx/bin/lmxd --transient --config /run/limanix-lmx/etc/lmx/config.json",
		"sudo /run/limanix-lmx/bin/lmx apply -g 0123456789ab --follow --json",
		"stop",
		"start",
		"sudo lmx status --wait converged -g 0123456789ab --json",
	}
	if !reflect.DeepEqual(client.calls, want) {
		t.Fatalf("guest steps:\n%s", strings.Join(client.calls, "\n"))
	}
	if stdout.String() != "/nix/store/00000000000000000000000000000000-nixos-system-dev-box-26.05\n" ||
		stderr.String() != "building the system configuration...\nthese 12 derivations will be built: …\n" {
		t.Fatalf("build lines: stdout %q, stderr %q", stdout.String(), stderr.String())
	}
	if len(warnings) != 2 || !strings.HasPrefix(warnings[0], "Less than 10% of the guest disk") || warnings[1] != "12 lines of the build output were skipped" {
		t.Fatalf("warnings: %q", warnings)
	}
}

func TestApplyReportsTheWaitAfterTheRestart(t *testing.T) {
	timeout := `{"contract":1,"ok":false,"error":{"code":"wait.timeout","message":"Generation 0123456789ab did not converge within 10 minutes.",` +
		`"details":{"conditions":[{"type":"RestartRequired","message":"Generation 0123456789ab is built; restart the VM to boot it."}]}}}`
	for _, test := range []struct {
		name       string
		wait       string
		want       string
		unfinished bool
	}{
		{"unfinalized", example(t, "wait-finalize-failed.json"), "Finalizing generation 0123456789ab failed: boot loader update failed; lmxd tries again later.", true},
		{"timeout", timeout, "lmx: Generation 0123456789ab did not converge within 10 minutes.\nGeneration 0123456789ab is built; restart the VM to boot it.", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			client := applyClient(example(t, "apply-follow.jsonl"), test.wait)
			guest := New(client)
			guest.Stdout, guest.Stderr, guest.Warn = io.Discard, io.Discard, func(string, ...any) {}

			err := guest.Apply(context.Background(), "sandbox", generation, domain.ARM64)
			if _, unfinished := errors.AsType[*FinalizeError](err); err == nil || err.Error() != test.want || unfinished != test.unfinished {
				t.Fatalf("wait: %v", err)
			}
			if slices.Contains(client.calls, "sudo systemctl start lmx.service") {
				t.Fatal("a booted generation was handed back to the old lmxd")
			}
		})
	}
}

func TestApplyFailureReturnsTheGuestToItsOwnLMXD(t *testing.T) {
	full := `{"contract":1,"ok":false,"error":{"code":"apply.build_failed","message":"nixos-rebuild failed with exit status 1.",` +
		`"details":{"exit_code":1,"disk":{"bytes":100,"free_bytes":2,"available_bytes":0,"inodes":10,"free_inodes":5}}}}` + "\n"
	for _, test := range []struct {
		name   string
		answer string
		disk   *DiskError
	}{
		{name: "build", answer: line(t, "apply-build-failed.json")},
		{name: "full disk", answer: full, disk: &DiskError{Usage: domain.DiskUsage{Bytes: 100, FreeBytes: 2, Inodes: 10, FreeInodes: 5}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			client := applyClient("", "")
			client.stream = func(_ context.Context, output io.Writer) error {
				_, _ = io.WriteString(output, test.answer)
				return errors.New("exit status 1")
			}

			err := New(client).Apply(context.Background(), "sandbox", generation, domain.ARM64)
			build, ok := errors.AsType[*Error](err)
			if !ok || build.Code != "apply.build_failed" || !strings.HasPrefix(err.Error(), "lmx: nixos-rebuild failed with exit status 1.") {
				t.Fatalf("build failure: %v", err)
			}
			if disk, _ := errors.AsType[*DiskError](err); !reflect.DeepEqual(disk, test.disk) {
				t.Fatalf("disk usage: %v", err)
			}
			restored := []string{"sudo systemctl stop lmx-transient.service", "sudo systemctl start lmx.service"}
			if !reflect.DeepEqual(client.calls[len(client.calls)-2:], restored) || slices.Contains(client.calls, "stop") {
				t.Fatalf("guest steps:\n%s", strings.Join(client.calls, "\n"))
			}
		})
	}
}

func TestApplyExplainsAFullDiskBeforeLMXRuns(t *testing.T) {
	failure := commandFailure(t, 1, "error: writing to file: No space left on device")
	client := applyClient("", "")
	client.run = func(_ context.Context, command string) (string, error) {
		if strings.HasPrefix(command, "sudo nix build") {
			return "", failure
		}
		return "", nil
	}

	err := New(client).Apply(context.Background(), "sandbox", generation, domain.ARM64)
	if !errors.Is(err, failure) || !errors.Is(err, errDiskFull) {
		t.Fatalf("full disk: %v", err)
	}
	if !slices.Contains(client.calls, "sudo systemctl start lmx.service") {
		t.Fatalf("guest steps:\n%s", strings.Join(client.calls, "\n"))
	}
}

func TestApplyCancellationStopsTheGuestApply(t *testing.T) {
	for _, confirmed := range []bool{true, false} {
		t.Run(fmt.Sprintf("confirmed=%t", confirmed), func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			stopped := make(chan struct{})
			client := applyClient("", "")
			client.stream = func(session context.Context, output io.Writer) error {
				cancel()
				select {
				case <-stopped:
					_, _ = io.WriteString(output, `{"contract":1,"ok":false,"error":{"code":"apply.cancelled","message":"The apply was cancelled."}}`+"\n")
					return errors.New("exit status 130")
				case <-session.Done():
					return session.Err()
				}
			}
			client.run = func(cleanup context.Context, command string) (string, error) {
				if !strings.Contains(command, "apply cancel") {
					return "", nil
				}
				if _, bounded := cleanup.Deadline(); !bounded || cleanup.Err() != nil {
					t.Errorf("the cancel has no context of its own: %v", cleanup.Err())
				}
				if !confirmed {
					return "", errors.New("guest unreachable")
				}
				close(stopped)
				return `{"contract":1,"ok":true,"data":{"cancelled":true}}`, nil
			}

			err := New(client).Apply(ctx, "sandbox", generation, domain.ARM64)
			if confirmed != errors.Is(err, context.Canceled) || !confirmed && !strings.Contains(err.Error(), "cannot confirm the guest apply stopped") {
				t.Fatalf("cancellation: %v", err)
			}
			if !slices.Contains(client.calls, "sudo /run/limanix-lmx/bin/lmx apply cancel -g 0123456789ab --json") ||
				!slices.Contains(client.calls, "sudo systemctl start lmx.service") || slices.Contains(client.calls, "stop") {
				t.Fatalf("guest steps:\n%s", strings.Join(client.calls, "\n"))
			}
		})
	}
}

func TestStatusReadsAddressAndDiskFromLMX(t *testing.T) {
	instance := lima.Instance{Name: "sandbox", Status: lima.Running, Networks: []lima.Network{{MACAddress: "52:55:55:AA:BB:CC", Shared: true}}}
	notice := Status{Notice: "the guest has no lmx yet; run limanix update"}
	for _, test := range []struct {
		name   string
		output string
		err    error
		want   Status
	}{
		{"complete", example(t, "status.json"), nil, Status{Address: "192.0.2.10", Disk: &domain.DiskUsage{Bytes: 17179869184, FreeBytes: 9663676416, Inodes: 1048576, FreeInodes: 495616}}},
		{"partial", example(t, "status-partial.json"), nil, Status{Disk: &domain.DiskUsage{Bytes: 17179869184, FreeBytes: 1073741824, Inodes: 1048576, FreeInodes: 52428}}},
		{"without lmx", "", commandFailure(t, 1, "sudo: lmx: command not found"), notice},
		{"before lmx", "", commandFailure(t, 2, "Usage: lmx {help|info|welcome}"), notice},
		{"unreachable", "", context.DeadlineExceeded, Status{}},
	} {
		t.Run(test.name, func(t *testing.T) {
			client := &fakeClient{run: func(ctx context.Context, _ string) (string, error) {
				if deadline, bounded := ctx.Deadline(); !bounded || time.Until(deadline) > statusTimeout {
					t.Error("the status probe has no deadline")
				}
				return test.output, test.err
			}}
			if status := New(client).Status(context.Background(), instance); !reflect.DeepEqual(status, test.want) {
				t.Fatalf("status: %+v", status)
			}
			if !reflect.DeepEqual(client.calls, []string{"sudo lmx status --json"}) {
				t.Fatalf("calls: %q", client.calls)
			}
		})
	}

	client := &fakeClient{}
	instance.Status = lima.Stopped
	if status := New(client).Status(context.Background(), instance); status != (Status{}) || len(client.calls) != 0 {
		t.Fatal("asked a stopped guest")
	}
}

func TestReserveWarnsOnlyAboutALowDisk(t *testing.T) {
	failed := errors.New("exit status 1")
	for _, test := range []struct {
		name   string
		output string
		err    error
		want   error
	}{
		{"room", example(t, "store-reserve.json"), nil, nil},
		{"low", example(t, "store-reserve-disk-low.json"), failed, &DiskError{Usage: domain.DiskUsage{Bytes: 17179869184, FreeBytes: 1342177280, Inodes: 1048576, FreeInodes: 98304}}},
		{"without lmx", "", commandFailure(t, 1, "sudo: lmx: command not found"), nil},
		{"without lmxd", `{"contract":1,"ok":false,"error":{"code":"owner.unavailable","message":"lmxd did not answer"}}`, failed, nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			client := &fakeClient{run: func(context.Context, string) (string, error) { return test.output, test.err }}
			if err := New(client).Reserve(context.Background(), "sandbox"); !reflect.DeepEqual(err, test.want) {
				t.Fatalf("reserve: %v", err)
			}
			if !reflect.DeepEqual(client.calls, []string{"sudo lmx store reserve --json"}) {
				t.Fatalf("calls: %q", client.calls)
			}
		})
	}
}

func TestChecksReadTheContractExamples(t *testing.T) {
	client := &fakeClient{run: func(ctx context.Context, command string) (string, error) {
		if deadline, bounded := ctx.Deadline(); !bounded || time.Until(deadline) > checkTimeout {
			t.Error("the checks have no deadline")
		}
		if strings.Contains(command, "net check") {
			// lmx exits with status 1 after a failed check, but answers.
			return line(t, "net-check.json"), commandFailure(t, 1, "")
		}
		return line(t, "doctor.json"), nil
	}}

	doctor, err := New(client).Doctor(context.Background(), "sandbox")
	restart := Check{Check: "generations", Status: CheckWarning, Message: "Generation 0123456789ab is built; restart the VM to boot it.", Hint: "Restart the VM from the Mac; limanix update does it."}
	if err != nil || len(doctor) != 3 || doctor[2] != restart {
		t.Fatalf("doctor: %+v %v", doctor, err)
	}
	port, err := New(client).NetCheck(context.Background(), "sandbox", 8080, false)
	if err != nil || len(port) != 3 || port[1].Check != "listener" || port[1].Status != CheckFailed || port[1].Hint == "" {
		t.Fatalf("net check: %+v %v", port, err)
	}
	if !reflect.DeepEqual(client.calls, []string{"sudo lmx doctor --json", "sudo lmx net check 8080 --json"}) {
		t.Fatalf("calls: %q", client.calls)
	}

	slow := &fakeClient{run: func(context.Context, string) (string, error) {
		return "", &lima.Error{Operation: "SSH", Err: context.DeadlineExceeded}
	}}
	if _, err := New(slow).Doctor(context.Background(), "sandbox"); err == nil || err.Error() != "lmx doctor gave no answer within 30s" {
		t.Fatalf("timeout: %v", err)
	}
}

func TestDecodeAnswerFollowsTheContract(t *testing.T) {
	failed := errors.New("exit status 1")
	for _, test := range []struct {
		output string
		want   string
	}{
		{`{"contract":2,"ok":true,"data":{}}`, "lmx answered with host contract 2; this LimaNix reads contract 1"},
		{`{"contract":1,"ok":true}`, "exit status 1"},
		{"Usage: lmx {help|info|welcome}", "exit status 1"},
		{`{"contract":1,"ok":false,"error":{"code":"future.code","message":"Something failed."}}`, "lmx: Something failed."},
	} {
		if err := decodeAnswer([]byte(test.output), failed, nil); err == nil || err.Error() != test.want {
			t.Errorf("%s: %v", test.output, err)
		}
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
	if !reflect.DeepEqual(client.calls, []string{"sudo --login --user dev"}) {
		t.Fatalf("interactive login must use the account shell once: %q", client.calls)
	}
}
