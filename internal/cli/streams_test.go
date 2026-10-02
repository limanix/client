package cli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestDefaultIOHelper(t *testing.T) {
	mode := os.Getenv("LIMANIX_DEFAULT_IO_HELPER")
	if mode == "" {
		return
	}
	manager := &fakeManager{failure: errors.New("fixture failure")}
	args := []string{"--help"}
	switch mode {
	case "usage":
		args = []string{"unknown-command"}
	case "failure":
		args = []string{"list"}
	}
	os.Exit(Execute(context.Background(), args, IO{}, fakeDependencies(manager, &fakeRegistry{})))
}

func TestOmittedStreamsInheritProcessStreamsWithoutPanicking(t *testing.T) {
	for _, test := range []struct {
		mode       string
		status     int
		diagnostic string
	}{
		{mode: "help"},
		{mode: "usage", status: 2, diagnostic: "accepts 0 arg(s)"},
		{mode: "failure", status: 1, diagnostic: "fixture failure"},
	} {
		t.Run(test.mode, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestDefaultIOHelper$")
			command.Env = append(os.Environ(), "LIMANIX_DEFAULT_IO_HELPER="+test.mode, "GORACE="+os.Getenv("GORACE")+" atexit_sleep_ms=0")
			var output, diagnostics bytes.Buffer
			command.Stdout = &output
			command.Stderr = &diagnostics
			err := command.Run()
			status := 0
			if exit, ok := errors.AsType[*exec.ExitError](err); ok {
				status = exit.ExitCode()
			} else if err != nil {
				t.Fatal(err)
			}
			if status != test.status || !strings.Contains(diagnostics.String(), test.diagnostic) || strings.Contains(diagnostics.String(), "panic:") {
				t.Fatalf("status=%d, output=%q, diagnostics=%q", status, output.String(), diagnostics.String())
			}
			if test.mode == "help" && !strings.Contains(output.String(), "Usage:") {
				t.Fatalf("help did not inherit stdout: %q", output.String())
			}
		})
	}
}
