package forward

import (
	"context"
	"errors"
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

const netstatTimeout = 5 * time.Second

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
// the port counts as taken.
func localListens(port uint16) bool {
	if runtime.GOOS == "darwin" {
		ctx, cancel := context.WithTimeout(context.Background(), netstatTimeout)
		defer cancel()

		output, err := exec.CommandContext(ctx, "netstat", "-an", "-p", "tcp").Output()

		return err != nil || netstatListens(string(output), port)
	}

	var tables strings.Builder

	for _, name := range []string{"/proc/net/tcp", "/proc/net/tcp6"} {
		// IPv6 may be off, which leaves no tcp6 table.
		if table, err := os.ReadFile(name); err == nil {
			tables.Write(table)
		} else if name == "/proc/net/tcp" {
			return true
		}
	}

	sockets, err := parseSockets(tables.String())

	return err != nil || slices.ContainsFunc(sockets, func(s socket) bool { return s.port == port })
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
