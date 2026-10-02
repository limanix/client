package hostagent

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestAPIClosePreservesSocketReplacement(t *testing.T) {
	directory, err := os.MkdirTemp("", "ha-sock-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(directory); err != nil {
			t.Error(err)
		}
	})
	socket := filepath.Join(directory, "ha.sock")
	api, err := startAPIServer(context.Background(), socket, nil, make(chan os.Signal, 1))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = api.server.Close() })
	if err := os.Remove(socket); err != nil {
		t.Fatal(err)
	}
	replacement, err := listenSocket(context.Background(), socket)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = replacement.Close() }()
	if err := api.Close(); !errors.Is(err, ErrReplacedSocket) {
		t.Fatalf("replaced socket not reported: %v", err)
	}
	info, err := os.Lstat(socket)
	if err != nil || info.Mode()&os.ModeSocket == 0 {
		t.Fatalf("shutdown removed replacement socket: %v", err)
	}
}
