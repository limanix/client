package forward

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"os/exec"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	netstatTimeout = 5 * time.Second
	netstatHeader  = "Local Address"
)

// inUse tells whether a program on the Mac may already own the port, without connecting to it: a connection would be
// accepted by a program that waits for one, such as a server that serves a single client.
//
// Binding the loopback addresses without SO_REUSEADDR fails when any socket holds the port, a listener on another
// address too, which a bind with SO_REUSEADDR may share on macOS and then take connections from. The same bind also
// fails for the TIME_WAIT left by a tunnel that was just in use, and a listener may reuse that. So when it fails,
// the system is asked whether a program listens.
func inUse(port uint16) bool {
	busy := false

	for _, address := range []struct{ network, host string }{{"tcp4", "127.0.0.1"}, {"tcp6", "::1"}} {
		err := bindExclusive(address.network, net.JoinHostPort(address.host, strconv.Itoa(int(port))))
		busy = busy || errors.Is(err, syscall.EADDRINUSE)
	}

	return busy && localListens(port)
}

func bindExclusive(network, address string) error {
	config := net.ListenConfig{Control: func(_, _ string, connection syscall.RawConn) error {
		var failure error

		if err := connection.Control(func(fd uintptr) {
			failure = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 0)
		}); err != nil {
			return err
		}

		return failure
	}}

	listener, err := config.Listen(context.Background(), network, address)
	if err != nil {
		return err
	}

	return listener.Close()
}

// localListens tells whether a program on the Mac listens on the port, on any address. When that cannot be told,
// the port counts as taken: a table that is empty or partial must not pass for one without the listener.
func localListens(port uint16) bool {
	var (
		listening bool
		err       error
	)

	if runtime.GOOS == "darwin" {
		listening, err = netstatListening("netstat", port)
	} else {
		listening, err = procListening(port, "/proc/net/tcp", "/proc/net/tcp6")
	}

	return err != nil || listening
}

// netstatListening runs `netstat -an -p tcp` of macOS. Its table has a header line, and netstat may exit with 0 and
// report a problem on stderr after printing nothing or a part of the table, which is no answer.
func netstatListening(command string, port uint16) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), netstatTimeout)
	defer cancel()

	var stderr strings.Builder

	netstat := exec.CommandContext(ctx, command, "-an", "-p", "tcp")
	netstat.Stderr = &stderr

	output, err := netstat.Output()
	if err != nil {
		return false, err
	}

	if reason := strings.TrimSpace(stderr.String()); reason != "" {
		return false, fmt.Errorf("netstat reported: %s", reason)
	}

	if !strings.Contains(string(output), netstatHeader) {
		return false, errors.New("netstat printed no socket table")
	}

	return netstatListens(string(output), port), nil
}

// procListening reads the socket tables of Linux. Without IPv6 there is no tcp6 table, but one that cannot be read is
// an error.
func procListening(port uint16, tcp, tcp6 string) (bool, error) {
	var tables strings.Builder

	for _, name := range []string{tcp, tcp6} {
		table, err := os.ReadFile(name)

		switch {
		case errors.Is(err, fs.ErrNotExist) && name == tcp6:
			continue
		case err != nil:
			return false, err
		}

		tables.Write(table)
	}

	sockets, err := parseSockets(tables.String())
	if err != nil {
		return false, err
	}

	return slices.ContainsFunc(sockets, func(s socket) bool { return s.port == port }), nil
}

// netstatListens reads `netstat -an -p tcp` of macOS, whose lines end with the state and show the local address as
// "*.3000", "127.0.0.1.3000" or "::1.3000".
func netstatListens(output string, port uint16) bool {
	suffix := "." + strconv.Itoa(int(port))

	for line := range strings.Lines(output) {
		fields := strings.Fields(line)
		if len(fields) >= 6 && fields[len(fields)-1] == "LISTEN" && strings.HasSuffix(fields[3], suffix) {
			return true
		}
	}

	return false
}
