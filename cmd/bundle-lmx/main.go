// bundle-lmx embeds locally built lmx release archives in a development build of Limanix.
//
// --source reads release archives from the checkout's dist directory and verifies their .sha256 files.
// It writes the archives and lmx.json to internal/nixos/resources/lmx for the generated guest flake.
// The version comes from the checkout's Cargo.toml.
//
// Without --source, it removes local bundles and uses the release pinned in resources/base/lmx.json.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/pelletier/go-toml/v2"

	"github.com/limanix/client/internal/filesystem"
)

// systems are the guest systems of the archives, as lmx.json names them.
var systems = []string{"aarch64-linux", "x86_64-linux"}

type archive struct {
	File   string `json:"file"`
	SHA256 string `json:"sha256"`
}

type pin struct {
	Version string             `json:"version"`
	Systems map[string]archive `json:"systems"`
}

func run(ctx context.Context, args []string, diagnostics io.Writer) int {
	var (
		flags = flag.NewFlagSet("bundle-lmx", flag.ContinueOnError)

		source = flags.String("source", "", "lmx checkout whose dist directory holds the release archives.")
		root   = flags.String("root", ".", "Repository root containing internal/nixos/resources.")
	)
	flags.SetOutput(diagnostics)

	err := flags.Parse(args)
	if errors.Is(err, flag.ErrHelp) {
		return 0
	}
	if err != nil {
		return 2
	}
	if flags.NArg() != 0 {
		_, _ = fmt.Fprintln(diagnostics, "bundle-lmx: positional arguments are not accepted")
		return 2
	}

	output := filepath.Join(*root, "internal", "nixos", "resources", "lmx")
	if *source == "" {
		err = os.RemoveAll(output)
	} else {
		err = bundle(ctx, *source, output, diagnostics)
	}
	if err != nil {
		_, _ = fmt.Fprintln(diagnostics, "bundle-lmx:", err)
		return 1
	}

	return 0
}

// bundle replaces output with the archives of the checkout's version and their lmx.json.
func bundle(ctx context.Context, source, output string, diagnostics io.Writer) error {
	version, err := checkoutVersion(source)
	if err != nil {
		return err
	}

	record := pin{Version: version, Systems: make(map[string]archive, len(systems))}
	contents := make(map[string][]byte, len(systems))
	for _, system := range systems {
		if err = ctx.Err(); err != nil {
			return err
		}

		name := fmt.Sprintf("lmx-%s-%s.tar.gz", version, system)
		data, sum, err := readArchive(filepath.Join(source, "dist", name))
		if err != nil {
			return err
		}

		contents[name] = data
		record.Systems[system] = archive{File: name, SHA256: sum}
	}

	encoded, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}

	if err = os.RemoveAll(output); err != nil {
		return err
	}
	if err = os.MkdirAll(output, 0o755); err != nil {
		return err
	}
	for name, data := range contents {
		if err = filesystem.WriteFileAtomic(filepath.Join(output, name), data, 0o644); err != nil {
			return err
		}
	}
	if err = filesystem.WriteFileAtomic(filepath.Join(output, "lmx.json"), append(encoded, '\n'), 0o644); err != nil {
		return err
	}

	_, err = fmt.Fprintf(diagnostics, "bundle-lmx: Prepared local lmx %s from %s.\n", version, source)
	return err
}

// checkoutVersion reads the workspace version that lmx's release archives carry in their names.
func checkoutVersion(source string) (string, error) {
	data, err := os.ReadFile(filepath.Join(source, "Cargo.toml"))
	if err != nil {
		return "", fmt.Errorf("read the lmx checkout: %w", err)
	}

	var manifest struct {
		Workspace struct {
			Package struct {
				Version string `toml:"version"`
			} `toml:"package"`
		} `toml:"workspace"`
	}
	if err = toml.Unmarshal(data, &manifest); err != nil {
		return "", fmt.Errorf("read %s: %w", filepath.Join(source, "Cargo.toml"), err)
	}

	version := manifest.Workspace.Package.Version
	if version == "" || strings.ContainsAny(version, "/\\ ") {
		return "", fmt.Errorf("%s has no workspace.package.version", filepath.Join(source, "Cargo.toml"))
	}

	return version, nil
}

// readArchive returns an archive and its sha256, which must match the digest file that the lmx build writes beside it.
func readArchive(path string) ([]byte, string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", fmt.Errorf("read %s; build it with task release/build in the lmx checkout: %w", path, err)
	}

	digest := sha256.Sum256(data)
	sum := hex.EncodeToString(digest[:])

	recorded, err := os.ReadFile(path + ".sha256")
	if err != nil {
		return nil, "", fmt.Errorf("read the digest of %s: %w", path, err)
	}
	fields := bytes.Fields(recorded)
	if len(fields) == 0 || string(fields[0]) != sum {
		return nil, "", fmt.Errorf("%s does not match %s.sha256", path, filepath.Base(path))
	}

	return data, sum, nil
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := run(ctx, os.Args[1:], os.Stderr)
	cancel()
	os.Exit(code)
}
