// build-docs assembles the documentation source tree used by the LimaNix docs site and client release.
//
// Run this command through task docs/prepare.
// The --root flag selects the repository directory.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/limanix/client/internal/docs/generator"
)

type options struct {
	root string
}

func parseOptions(args []string, diagnostics io.Writer) (options, error) {
	var (
		flags = flag.NewFlagSet("build-docs", flag.ContinueOnError)
		opts  options
	)

	flags.SetOutput(diagnostics)
	flags.StringVar(&opts.root, "root", ".", "Repository root containing guides/ and the build/docs output.")

	if err := flags.Parse(args); err != nil {
		return options{}, err
	}
	if flags.NArg() != 0 {
		err := errors.New("unexpected positional arguments")
		_, _ = fmt.Fprintln(diagnostics, err)
		flags.Usage()
		return options{}, err
	}
	return opts, nil
}

func run(ctx context.Context, args []string, diagnostics io.Writer) int {
	opts, err := parseOptions(args, diagnostics)
	if errors.Is(err, flag.ErrHelp) {
		return 0
	}
	if err != nil {
		return 2
	}

	if err = generator.Generate(ctx, opts.root, diagnostics); err != nil {
		_, _ = fmt.Fprintln(diagnostics, "build-docs:", err)
		return 1
	}
	return 0
}

func main() {
	var (
		ctx, cancel = signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		code        = run(ctx, os.Args[1:], os.Stderr)
	)

	cancel()
	os.Exit(code)
}
