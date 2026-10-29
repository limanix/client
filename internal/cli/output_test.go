package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"testing"

	"github.com/limanix/client/internal/domain"
	"github.com/limanix/client/internal/lima"
	"github.com/limanix/client/internal/vm"
)

func TestLifecycleJSONFetchesOnlyTarget(t *testing.T) {
	ready, stopped := domain.Ready, lima.Stopped
	home := "/managed/home"
	want := vm.Info{Name: "sandbox", OperationStatus: &ready, BackendStatus: &stopped, Home: &home}

	for _, test := range []struct {
		args []string
		call managerCall
	}{
		{[]string{"create", "--config=cfg.toml", "--json"}, managerCall{operation: "create", path: "cfg.toml"}},
		{[]string{"update", "--config=cfg.toml", "--json"}, managerCall{operation: "update", path: "cfg.toml"}},
		{[]string{"start", "sandbox", "--json"}, managerCall{operation: "start", name: "sandbox"}},
		{[]string{"stop", "sandbox", "--json"}, managerCall{operation: "stop", name: "sandbox"}},
	} {
		t.Run(test.args[0], func(t *testing.T) {
			manager := &fakeManager{entries: []vm.Info{{Name: "unrelated"}, want}}
			status, output, diagnostics := runCLI(test.args, fakeDependencies(manager, &fakeRegistry{}))
			if status != 0 || diagnostics != "" {
				t.Fatalf("command failed: %d %q", status, diagnostics)
			}
			var result vm.Info
			if err := json.Unmarshal([]byte(output), &result); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(result, want) {
				t.Fatalf("VM JSON changed: got %#v, want %#v", result, want)
			}
			calls := []managerCall{test.call, {operation: "fetch", name: "sandbox"}}
			if !reflect.DeepEqual(manager.calls, calls) {
				t.Fatalf("command queried other VMs: %#v", manager.calls)
			}
		})
	}
}

func TestLifecycleJSONFetchErrorsKeepExitStatusAndDiagnostics(t *testing.T) {
	for _, test := range []struct {
		name        string
		err         error
		status      int
		diagnostics string
	}{
		{"read failure", errors.New("cannot read VM state"), 1, "limanix: cannot read VM state\n"},
		{"canceled", context.Canceled, 130, "limanix: interrupted.\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			manager := &fakeManager{fetch: func(context.Context, domain.VMName) (vm.Info, error) {
				return vm.Info{}, test.err
			}}
			status, output, diagnostics := runCLI([]string{"stop", "sandbox", "--json"}, fakeDependencies(manager, &fakeRegistry{}))
			if status != test.status || output != "" || diagnostics != test.diagnostics {
				t.Fatalf("fetch error changed: %d %q %q", status, output, diagnostics)
			}
		})
	}
}

func TestLifecycleJSONReportsWriteFailure(t *testing.T) {
	reader, writer := io.Pipe()
	t.Cleanup(func() {
		if err := writer.Close(); err != nil {
			t.Error(err)
		}
	})
	if err := reader.CloseWithError(errors.New("output closed")); err != nil {
		t.Fatal(err)
	}

	manager := &fakeManager{entries: []vm.Info{{Name: "sandbox"}}}
	var diagnostics bytes.Buffer
	status := Execute(context.Background(), []string{"stop", "sandbox", "--json"},
		IO{Out: writer, Err: &diagnostics}, fakeDependencies(manager, &fakeRegistry{}))
	if status != 1 || diagnostics.String() != "limanix: output closed\n" {
		t.Fatalf("write error changed: %d %q", status, diagnostics.String())
	}
}
