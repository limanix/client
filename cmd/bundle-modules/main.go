// bundle-modules prepares the standard NixOS modules embedded in Limanix.
//
// It downloads a limanix/modules tag or packages a local catalog checkout. The nixos package embeds the archive in Limanix,
// making its modules available for VM configuration when needed.
//
// The --version flag selects the exact Git tag; --root selects the repository.
// Existing valid output for that tag is reused without a download.
// --source selects a local catalog directory; --version supplies its explicit build label.
// Local source is always checked and repacked; identical output is reused.
//
// Upstream: https://github.com/limanix/modules.
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

	"github.com/limanix/client/internal/nixos/modulegen"
)

func run(ctx context.Context, args []string, diagnostics io.Writer) int {
	var (
		flags = flag.NewFlagSet("bundle-modules", flag.ContinueOnError)

		version = flags.String("version", "", "Required Git release tag or local source build label.")
		source  = flags.String("source", "", "Local catalog repository to package instead of downloading a tag.")
		root    = flags.String("root", ".", "Repository root containing internal/nixos/resources.")
	)
	flags.SetOutput(diagnostics)

	err := flags.Parse(args)
	if errors.Is(err, flag.ErrHelp) {
		return 0
	}
	if err != nil {
		return 2
	}
	if flags.NArg() != 0 || *version == "" {
		_, _ = fmt.Fprintln(diagnostics, "bundle-modules: --version is required; positional arguments are not accepted")
		return 2
	}

	if *source == "" {
		err = modulegen.Generate(ctx, *root, *version, diagnostics)
	} else {
		err = modulegen.GenerateLocal(ctx, *root, *source, *version, diagnostics)
	}
	if err != nil {
		_, _ = fmt.Fprintln(diagnostics, "bundle-modules:", err)
		return 1
	}

	return 0
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := run(ctx, os.Args[1:], os.Stderr)
	cancel()
	os.Exit(code)
}
