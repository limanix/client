package hostagent

import (
	"context"
	"errors"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	limaagent "github.com/lima-vm/lima/v2/pkg/hostagent"
	"github.com/lima-vm/lima/v2/pkg/hostagent/api/server"
)

type apiServer struct {
	server     *http.Server
	socket     string
	socketInfo fs.FileInfo
	result     <-chan error
}

type socketListener struct {
	*net.UnixListener
	info fs.FileInfo
}

// Close stops serving, waits for Serve to finish, and removes the checked socket.
func (api *apiServer) Close() error {
	closeErr := api.server.Close()
	serveErr := <-api.result
	if errors.Is(serveErr, http.ErrServerClosed) {
		serveErr = nil
	}

	return errors.Join(closeErr, serveErr, removeOwnedSocket(api.socket, api.socketInfo))
}

func startAPIServer(ctx context.Context, socket string, agent *limaagent.HostAgent, signals chan<- os.Signal) (*apiServer, error) {
	if err := removeSocket(socket); err != nil {
		return nil, err
	}

	listener, err := listenSocket(ctx, socket)
	if err != nil {
		return nil, err
	}

	mux := http.NewServeMux()
	server.AddRoutes(mux, &server.Backend{Agent: agent})

	var (
		httpServer = &http.Server{
			Handler:           mux,
			ReadHeaderTimeout: 10 * time.Second,
		}
		result = make(chan error, 1)
	)

	go func() {
		err = httpServer.Serve(listener)
		result <- err

		if !errors.Is(err, http.ErrServerClosed) {
			requestShutdown(signals)
		}
	}()

	return &apiServer{
		server:     httpServer,
		socket:     socket,
		socketInfo: listener.info,
		result:     result,
	}, nil
}

func listenSocket(ctx context.Context, socket string) (*socketListener, error) {
	if err := requirePrivateDirectory(filepath.Dir(socket)); err != nil {
		return nil, err
	}

	var listenConfig net.ListenConfig

	listener, err := listenConfig.Listen(ctx, "unix", socket)
	if err != nil {
		return nil, err
	}

	unixListener := listener.(*net.UnixListener)
	unixListener.SetUnlinkOnClose(false)
	info, err := os.Lstat(socket)
	if err != nil {
		return nil, errors.Join(err, unixListener.Close())
	}
	if info.Mode()&os.ModeSocket == 0 {
		return nil, errors.Join(ErrOccupiedSocket, unixListener.Close())
	}
	if err = os.Chmod(socket, 0o600); err != nil {
		return nil, errors.Join(err, unixListener.Close(), removeOwnedSocket(socket, info))
	}

	return &socketListener{UnixListener: unixListener, info: info}, nil
}

func removeSocket(filename string) error {
	info, err := os.Lstat(filename)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}

	if err != nil {
		return err
	}

	if info.Mode()&os.ModeSocket == 0 {
		return ErrOccupiedSocket
	}

	return os.Remove(filename)
}

// removeOwnedSocket preserves another socket that replaced this listener's path.
func removeOwnedSocket(filename string, original fs.FileInfo) error {
	info, err := os.Lstat(filename)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSocket == 0 || original == nil || !os.SameFile(info, original) {
		return ErrReplacedSocket
	}
	return os.Remove(filename)
}
