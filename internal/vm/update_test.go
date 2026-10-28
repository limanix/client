package vm

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/limanix/client/internal/domain"
	"github.com/limanix/client/internal/lima"
)

func TestUpdateBuildsBeforeSelectingAndRestarting(t *testing.T) {
	f := fixture(t)
	first, _ := f.create(t, "sandbox")
	name := first.Identity.LimaName()
	directory, err := f.store.GenerationDir(first)
	if err != nil {
		t.Fatal(err)
	}
	current := filepath.Join(filepath.Dir(directory), "current")
	cfg := f.configuration("sandbox")
	f.backend.onBuild = func() {
		if *f.backend.instances[name].Memory != int64(cfg.Resources.Mem) {
			t.Fatal("memory increase was not applied before building")
		}
		if target, err := os.Readlink(current); err != nil || target != first.Generation {
			t.Fatalf("current changed before build succeeded: %q: %v", target, err)
		}
	}
	cfg.Resources.CPU++
	f.backend.calls = nil
	f.backend.failOn = "stream"
	if _, err = f.manager.Update(context.Background(), f.writeConfig(t, cfg)); err == nil {
		t.Fatal("failed build unexpectedly succeeded")
	}
	if f.backend.instances[name].Status != lima.Running || !slices.Equal(updateEffects(f.backend.calls), []string{"stream"}) {
		t.Fatalf("failed build changed Lima: %v", f.backend.calls)
	}
	if target, err := os.Readlink(current); err != nil || target != first.Generation {
		t.Fatalf("failed build changed current: %q: %v", target, err)
	}

	cfg.Resources.Mem += domain.ByteSize(domain.GiB)
	f.backend.calls, f.backend.failOn = nil, ""
	updated, err := f.manager.Update(context.Background(), f.writeConfig(t, cfg))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"stop", "edit", "start", "stream", "stop", "edit", "start"}
	if got := updateEffects(f.backend.calls); !slices.Equal(got, want) {
		t.Fatalf("update order: got %v, want %v", got, want)
	}
	if target, err := os.Readlink(current); err != nil || target != updated.Generation {
		t.Fatalf("successful build not selected: %q: %v", target, err)
	}
}

func updateEffects(calls []backendCall) []string {
	var operations []string
	for _, call := range calls {
		if slices.Contains([]string{"stream", "stop", "edit", "start"}, call.Operation) {
			operations = append(operations, call.Operation)
		}
	}
	return operations
}
