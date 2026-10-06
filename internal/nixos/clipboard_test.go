package nixos

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// clipboardProcess runs pbcopy.sh or pbpaste.sh with the terminal replaced by files and PATH by test tools.
func clipboardProcess(t *testing.T, name string, env []string, args ...string) *exec.Cmd {
	t.Helper()
	script, err := resources.ReadFile("resources/base/" + name + ".sh")
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command(workspaceExecutable(t, "bash"), append([]string{"-c", string(script), name}, args...)...)
	command.Env = append([]string{
		"PATH=/usr/bin:/bin",
		"limanix_base64=" + workspaceExecutable(t, "base64"),
		"limanix_paste_timeout=1",
	}, env...)
	return command
}

func TestPbcopySendsInputToTheMacClipboard(t *testing.T) {
	terminal := filepath.Join(t.TempDir(), "tty")
	command := clipboardProcess(t, "pbcopy", []string{"limanix_tty_out=" + terminal})
	command.Stdin = strings.NewReader("hello\n")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("pbcopy failed: %v %s", err, output)
	}
	if data, err := os.ReadFile(terminal); err != nil || string(data) != "\x1b]52;c;aGVsbG8K\a" {
		t.Fatalf("terminal did not receive OSC 52: %q %v", data, err)
	}

	command = clipboardProcess(t, "pbcopy", []string{"limanix_tty_out=" + filepath.Join(t.TempDir(), "missing", "tty")})
	command.Stdin = strings.NewReader("hello\n")
	if output, err := command.CombinedOutput(); err == nil || !strings.Contains(string(output), "terminal") {
		t.Fatalf("missing terminal must fail clearly: %v %s", err, output)
	}
}

func TestPbcopyInTmuxUsesTheAttachedClient(t *testing.T) {
	directory := t.TempDir()
	tmux := workspaceTool(t, "tmux", `printf '%s\n' "$*" > "$CLIP_DIR/args"
cat > "$CLIP_DIR/stdin"
`)
	command := clipboardProcess(t, "pbcopy", []string{
		"PATH=" + filepath.Dir(tmux) + ":/usr/bin:/bin",
		"TMUX=/tmp/tmux-501/default,1,0",
		"CLIP_DIR=" + directory,
		"limanix_tty_out=" + filepath.Join(directory, "unused"),
	})
	command.Stdin = strings.NewReader("hello\n")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("pbcopy failed in tmux: %v %s", err, output)
	}
	args, _ := os.ReadFile(filepath.Join(directory, "args"))
	stdin, _ := os.ReadFile(filepath.Join(directory, "stdin"))
	if string(args) != "load-buffer -w -\n" || string(stdin) != "hello\n" {
		t.Fatalf("tmux must load and forward the buffer: %q %q", args, stdin)
	}
	if _, err := os.Stat(filepath.Join(directory, "unused")); err == nil {
		t.Fatal("pbcopy bypassed tmux")
	}
}

func TestPbpasteReadsTheMacClipboardThroughTheTerminal(t *testing.T) {
	directory := t.TempDir()
	reply, query := filepath.Join(directory, "reply"), filepath.Join(directory, "query")
	if err := os.WriteFile(reply, []byte("\x1b]52;c;aGVsbG8gd29ybGQ=\a"), 0o600); err != nil {
		t.Fatal(err)
	}
	stty := workspaceTool(t, "stty", `printf '%s\n' "$*" >> "$CLIP_DIR/stty"
[ "$1" != -g ] || printf '%s\n' saved
`)
	env := []string{"CLIP_DIR=" + directory, "limanix_stty=" + stty, "limanix_tty_in=" + reply, "limanix_tty_out=" + query}
	output, err := clipboardProcess(t, "pbpaste", env).Output()
	if err != nil || string(output) != "hello world" {
		t.Fatalf("clipboard text not returned: %q %v", output, err)
	}
	if data, _ := os.ReadFile(query); string(data) != "\x1b]52;c;?\a" {
		t.Fatalf("terminal was not asked for the clipboard: %q", data)
	}
	if modes, _ := os.ReadFile(filepath.Join(directory, "stty")); !bytes.HasSuffix(modes, []byte("saved\n")) {
		t.Fatalf("terminal mode not restored: %q", modes)
	}

	if err = os.WriteFile(reply, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	command := clipboardProcess(t, "pbpaste", env)
	if output, err = command.CombinedOutput(); err == nil || !strings.Contains(string(output), "Cmd+V") {
		t.Fatalf("an unanswered query must explain the fallback: %v %s", err, output)
	}
}

func TestPbpasteInTmuxWaitsForTheRequestedBuffer(t *testing.T) {
	directory := t.TempDir()
	tmux := workspaceTool(t, "tmux", `case "$1" in
  list-buffers) if [ -e "$CLIP_DIR/requested" ]; then echo '200 buffer1'; else echo '100 buffer0'; fi ;;
  refresh-client) printf '%s\n' "$*" > "$CLIP_DIR/requested" ;;
  save-buffer) printf 'from mac' ;;
esac
`)
	command := clipboardProcess(t, "pbpaste", []string{
		"PATH=" + filepath.Dir(tmux) + ":/usr/bin:/bin",
		"TMUX=/tmp/tmux-501/default,1,0",
		"CLIP_DIR=" + directory,
	})
	output, err := command.Output()
	if err != nil || string(output) != "from mac" {
		t.Fatalf("tmux clipboard request failed: %q %v", output, err)
	}
	if request, _ := os.ReadFile(filepath.Join(directory, "requested")); string(request) != "refresh-client -l\n" {
		t.Fatalf("tmux was not asked to request the clipboard: %q", request)
	}
}

func TestClipboardCommandsRejectArguments(t *testing.T) {
	for _, name := range []string{"pbcopy", "pbpaste"} {
		command := clipboardProcess(t, name, nil, "extra")
		var exit *exec.ExitError
		if output, err := command.CombinedOutput(); !errors.As(err, &exit) || exit.ExitCode() != 2 || !strings.Contains(string(output), "Usage: "+name) {
			t.Fatalf("%s accepted arguments: %v %s", name, err, output)
		}
	}
}
