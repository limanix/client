package nixos

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// listedPackage is one line of the LISTED file of packages-check.sh.
type listedPackage struct {
	name      string
	directory string
}

// storePackage creates a directory named like a store path, with the given commands in bin/.
func storePackage(t *testing.T, root, name string, commands ...string) string {
	t.Helper()
	directory := filepath.Join(root, strings.Repeat("a", 32)+"-"+name)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	if len(commands) > 0 {
		if err := os.MkdirAll(filepath.Join(directory, "bin"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, command := range commands {
		if err := os.WriteFile(filepath.Join(directory, "bin", command), []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return directory
}

// runPackagesCheck runs packages-check.sh and returns its exit status and standard error.
func runPackagesCheck(t *testing.T, listed []listedPackage, others []string) (int, string) {
	t.Helper()
	root := t.TempDir()
	var lines strings.Builder
	for _, entry := range listed {
		lines.WriteString(entry.name + "\t" + entry.directory + "\n")
	}
	listedFile := filepath.Join(root, "listed")
	othersFile := filepath.Join(root, "others")
	if err := os.WriteFile(listedFile, []byte(lines.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	var otherLines string
	if len(others) > 0 {
		otherLines = strings.Join(others, "\n") + "\n"
	}
	if err := os.WriteFile(othersFile, []byte(otherLines), 0o600); err != nil {
		t.Fatal(err)
	}

	command := exec.Command("sh", filepath.Join("resources", "base", "packages-check.sh"), listedFile, othersFile)
	var stderr strings.Builder
	command.Stderr = &stderr
	err := command.Run()
	if exit, ok := errors.AsType[*exec.ExitError](err); ok {
		return exit.ExitCode(), stderr.String()
	}
	if err != nil {
		t.Fatal(err)
	}
	return 0, stderr.String()
}

func TestPackagesCheckRejectsACommandOfAnotherPackage(t *testing.T) {
	root := t.TempDir()
	module := storePackage(t, root, "neovim-0.11.4", "nvim")
	listed := storePackage(t, root, "neovim-0.11.5", "nvim")

	status, stderr := runPackagesCheck(t, []listedPackage{{"neovim", listed}}, []string{module})
	expected := "nixos.packages: neovim provides nvim, which neovim-0.11.4 (from a module or the platform) also provides; remove it from packages, or choose one in a NixOS module\n"
	if status != 1 || stderr != expected {
		t.Fatalf("status %d, stderr %q", status, stderr)
	}
}

func TestPackagesCheckRejectsTwoListedPackagesWithOneCommand(t *testing.T) {
	root := t.TempDir()
	older := storePackage(t, root, "python3-3.12.11", "python3", "python3.12")
	newer := storePackage(t, root, "python3-3.14.0", "python3", "python3.14")

	status, stderr := runPackagesCheck(t, []listedPackage{{"python312", older}, {"python314", newer}}, nil)
	expected := "nixos.packages: python312 and python314 both provide python3; keep one, or use a module that supports several versions\n"
	if status != 1 || stderr != expected {
		t.Fatalf("status %d, stderr %q", status, stderr)
	}
}

func TestPackagesCheckAcceptsTheSamePackage(t *testing.T) {
	git := storePackage(t, t.TempDir(), "git-2.51.0", "git", "git-shell")

	if status, stderr := runPackagesCheck(t, []listedPackage{{"git", git}}, []string{git}); status != 0 || stderr != "" {
		t.Fatalf("status %d, stderr %q", status, stderr)
	}
}

func TestPackagesCheckIgnoresPackagesWithoutCommands(t *testing.T) {
	root := t.TempDir()
	library := storePackage(t, root, "python3.13-requests-2.32.5")
	tools := storePackage(t, root, "coreutils-full-9.8", "ls")

	if status, stderr := runPackagesCheck(t, []listedPackage{{"python3Packages.requests", library}}, []string{tools}); status != 0 || stderr != "" {
		t.Fatalf("status %d, stderr %q", status, stderr)
	}
}

func TestPackagesCheckIgnoresConflictsBetweenOtherPackages(t *testing.T) {
	root := t.TempDir()
	gcc := storePackage(t, root, "gcc-wrapper-14.3.0", "cc")
	clang := storePackage(t, root, "clang-wrapper-20.1.8", "cc")
	jq := storePackage(t, root, "jq-1.8.1-bin", "jq")

	if status, stderr := runPackagesCheck(t, []listedPackage{{"jq", jq}}, []string{gcc, clang}); status != 0 || stderr != "" {
		t.Fatalf("status %d, stderr %q", status, stderr)
	}
}

func TestPackagesCheckReportsEveryConflict(t *testing.T) {
	root := t.TempDir()
	system := storePackage(t, root, "coreutils-full-9.8", "cat", "ls")
	listed := storePackage(t, root, "coreutils-9.8", "cat", "ls")

	status, stderr := runPackagesCheck(t, []listedPackage{{"coreutils", listed}}, []string{system})
	if lines := strings.Split(strings.TrimSpace(stderr), "\n"); status != 1 || len(lines) != 2 {
		t.Fatalf("status %d, stderr %q", status, stderr)
	}
}
