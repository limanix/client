package nixos

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/limanix/client/internal/config"
	"github.com/limanix/client/internal/domain"
)

func TestRuntimeCarriesGuestDiskPolicy(t *testing.T) {
	cfg := config.Default()
	cfg.Resources.Disk = domain.ByteSize(16 * domain.GiB)
	flake, err := Prepare(cfg, filepath.Join(t.TempDir(), "runtime"), nil, 501)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(flake, "runtime.json"))
	if err != nil {
		t.Fatal(err)
	}
	var runtime struct {
		Disk struct {
			Bytes          int64  `json:"bytes"`
			CollectPercent uint64 `json:"collectPercent"`
			MinimumPercent uint64 `json:"minimumPercent"`
		} `json:"disk"`
	}
	if err = json.Unmarshal(data, &runtime); err != nil {
		t.Fatal(err)
	}
	if runtime.Disk.Bytes != 16*domain.GiB || runtime.Disk.CollectPercent != domain.DiskCollectPercent ||
		runtime.Disk.MinimumPercent != domain.DiskMinimumPercent {
		t.Fatalf("guest disk policy missing from runtime: %s", data)
	}
}

// storeGuard runs store-guard.sh with a stat that answers usages in order and a nix-store that logs its arguments.
func storeGuard(t *testing.T, usages ...string) (output, calls string, err error) {
	t.Helper()
	script, err := resources.ReadFile("resources/base/store-guard.sh")
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	if err = os.WriteFile(filepath.Join(directory, "usages"), []byte(strings.Join(usages, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	stat := workspaceTool(t, "stat", `count=$(cat "$GUARD_DIR/count" 2>/dev/null || echo 0)
count=$((count + 1))
echo "$count" > "$GUARD_DIR/count"
line=$(sed -n "${count}p" "$GUARD_DIR/usages")
[ -n "$line" ] || exit 1
printf '%s\n' "$line"
`)
	store := workspaceTool(t, "nix-store", `printf '%s\n' "$*" >> "$GUARD_DIR/calls"
case "$*" in
  *--print-roots*) cat <<'ROOTS'
"/proc/1/maps" -> /nix/store/aaaa-glibc
"/run/current-system" -> /nix/store/bbbb-nixos-system
"/nix/var/nix/profiles/system-4-link" -> /nix/store/bbbb-nixos-system
"/home/dev/project/.direnv/flake-inputs/cccc-source" -> /nix/store/cccc-source
ROOTS
  ;;
esac
`)
	command := exec.Command(workspaceExecutable(t, "bash"), "-c", string(script), "store-guard")
	command.Env = []string{
		"PATH=/usr/bin:/bin",
		"GUARD_DIR=" + directory,
		"limanix_stat=" + stat,
		"limanix_nix_store=" + store,
		"limanix_grep=" + workspaceExecutable(t, "grep"),
		"limanix_collect_percent=20",
		"limanix_minimum_percent=10",
	}
	data, err := command.CombinedOutput()
	logged, _ := os.ReadFile(filepath.Join(directory, "calls"))
	return string(data), string(logged), err
}

func TestStoreGuardCollectsOnlyBelowHeadroom(t *testing.T) {
	const healthy, low, critical = "1000 500 1000 500", "1000 500 1000 150", "1000 500 1000 50"
	output, calls, err := storeGuard(t, healthy)
	if err != nil || calls != "" {
		t.Fatalf("healthy disk was collected: %v %q %q", err, calls, output)
	}
	output, calls, err = storeGuard(t, low, healthy)
	if err != nil || calls != "--gc --quiet\n" || strings.Contains(output, "roots") {
		t.Fatalf("low disk must be collected once: %v %q %q", err, calls, output)
	}
	output, calls, err = storeGuard(t, critical, critical)
	if err != nil || calls != "--gc --quiet\n--gc --print-roots\n" {
		t.Fatalf("roots must be reported when collection cannot restore the minimum: %v %q %q", err, calls, output)
	}
	if !strings.Contains(output, "/home/dev/project/.direnv/flake-inputs/cccc-source") ||
		strings.Contains(output, "/proc/") || strings.Contains(output, "/run/") || strings.Contains(output, "profiles/system") {
		t.Fatalf("only roots outside the system belong in the report: %q", output)
	}
	if _, calls, err = storeGuard(t); err == nil || calls != "" {
		t.Fatalf("unreadable usage must fail without collecting: %v %q", err, calls)
	}
}
