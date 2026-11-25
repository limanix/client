// Package forward brings ports of a VM to localhost on the Mac over one SSH connection.
package forward

import (
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"math"
	"net"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang.org/x/crypto/ssh"
)

const (
	pollInterval    = time.Second
	spareLocalPorts = 100
	socketTables    = "/run/current-system/sw/bin/cat /proc/net/tcp /proc/net/tcp6"
	tableHeader     = "local_address"
)

var systemPorts = []uint16{22, 53}

// ErrConnectionLost ends Run when the SSH connection to the VM closes.
var ErrConnectionLost = errors.New("the connection to the VM was lost")

// Port maps a port of the Mac to a port of the VM.
type Port struct {
	Local  uint16
	Remote uint16
}

type tunnel struct {
	remote    uint16
	local     uint16
	listeners []net.Listener

	mu     sync.Mutex
	target string
}

func (t *tunnel) destination() string {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.target
}

// retarget points new connections to the first of listening unless the tunnel already reaches one of them, so that a
// port that is open on several addresses does not make the destination flap between polls.
func (t *tunnel) retarget(listening []string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !slices.Contains(listening, t.target) {
		t.target = listening[0]
	}
}

func (t *tunnel) close() {
	for _, listener := range t.listeners {
		_ = listener.Close()
	}
}

type forwarder struct {
	client  *ssh.Client
	out     io.Writer
	tunnels []*tunnel

	// refused holds the ports of the VM that have no tunnel because the Mac has no port for them, so that the warning
	// is printed once while the conflict lasts.
	refused map[uint16]bool
}

// Run forwards ports until ctx ends or the connection is lost. Without ports, it forwards every port that a program
// in the VM listens on while it listens, to the same port on the Mac or the next free one. It returns why it stopped:
// the cause of ctx, such as context.Canceled for an interrupt, or ErrConnectionLost.
func Run(ctx context.Context, client *ssh.Client, ports []Port, out io.Writer) error {
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)

	stop := context.AfterFunc(ctx, func() { _ = client.Close() })
	defer stop()

	go func() {
		_ = client.Wait()
		cancel(ErrConnectionLost)
	}()

	f := &forwarder{client: client, out: out}
	defer f.closeAll()

	var err error
	if len(ports) == 0 {
		err = f.discover(ctx)
	} else {
		err = f.fixed(ctx, ports)
	}

	if err != nil {
		return err
	}

	// Both modes return nil only when ctx ends, with context.Canceled for an interrupt, as other commands report it.
	return context.Cause(ctx)
}

func (f *forwarder) fixed(ctx context.Context, ports []Port) error {
	for _, port := range ports {
		target := net.JoinHostPort("localhost", strconv.Itoa(int(port.Remote)))
		if err := f.open(port.Remote, port.Local, target, false); err != nil {
			return err
		}
	}

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		// A program may listen on one address of the VM only, and "localhost" does not reach it. Until the program
		// listens, and when the poll fails, the tunnel keeps the address it has.
		if sockets, err := f.sockets(); err == nil {
			listening := listeningTargets(sockets)

			for _, t := range f.tunnels {
				if targets, ok := listening[t.remote]; ok {
					t.retarget(targets)
				}
			}
		}

		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func (f *forwarder) discover(ctx context.Context) error {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	polled, failing := false, false

	for {
		sockets, err := f.sockets()

		var exit *ssh.ExitError

		switch {
		case err == nil:
			polled, failing = true, false

			if err = f.sync(sockets); err != nil {
				return err
			}
		case ctx.Err() != nil:
			return nil
		case polled && errors.As(err, &exit):
			// A failed poll says nothing about the services, so the open ports stay until a poll succeeds.
			if !failing {
				_, _ = fmt.Fprintf(f.out, "%v; keeping the open ports and trying again\n", err)
			}

			failing = true
		default:
			return err
		}

		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

// sockets lists the listening sockets of the VM. A failed command is an error, never an empty list: an empty list means
// that nothing listens, and makes sync close every tunnel.
func (f *forwarder) sockets() ([]socket, error) {
	session, err := f.client.NewSession()
	if err != nil {
		return nil, err
	}
	defer func() { _ = session.Close() }()

	var stderr strings.Builder

	session.Stderr = &stderr

	output, err := session.Output(socketTables)

	// Without /proc/net/tcp6, i.e. with IPv6 off, cat prints the IPv4 table and still exits with 1.
	var exit *ssh.ExitError
	if err != nil && !(errors.As(err, &exit) && strings.Contains(string(output), tableHeader)) {
		if reason := strings.TrimSpace(stderr.String()); reason != "" {
			err = fmt.Errorf("%w: %s", err, reason)
		}

		return nil, fmt.Errorf("read the sockets of the VM: %w", err)
	}

	return parseSockets(string(output))
}

// listeningTargets maps each port that a program in the VM listens on to the addresses to connect to.
func listeningTargets(sockets []socket) map[uint16][]string {
	targets := map[uint16][]string{}

	for _, s := range sockets {
		if target := s.target(); !slices.Contains(targets[s.port], target) {
			targets[s.port] = append(targets[s.port], target)
		}
	}

	return targets
}

func (f *forwarder) sync(sockets []socket) error {
	targets := listeningTargets(sockets)

	for _, port := range systemPorts {
		delete(targets, port)
	}

	if f.refused == nil {
		f.refused = map[uint16]bool{}
	}

	for remote := range f.refused {
		if _, listening := targets[remote]; !listening {
			delete(f.refused, remote)
		}
	}

	f.tunnels = slices.DeleteFunc(f.tunnels, func(t *tunnel) bool {
		if listening, ok := targets[t.remote]; ok {
			t.retarget(listening)
			return false
		}

		t.close()
		_, _ = fmt.Fprintf(f.out, "localhost:%d closed\n", t.local)

		return true
	})

	for _, remote := range slices.Sorted(maps.Keys(targets)) {
		forwarded := slices.ContainsFunc(f.tunnels, func(t *tunnel) bool { return t.remote == remote })
		if forwarded {
			continue
		}

		// One port without a local port must not take the other tunnels down: they stay, and the port is tried again
		// on the next poll.
		if err := f.open(remote, remote, targets[remote][0], true); err != nil {
			if !f.refused[remote] {
				_, _ = fmt.Fprintf(f.out, "VM port %d is not forwarded: %v\n", remote, err)
			}

			f.refused[remote] = true

			continue
		}

		delete(f.refused, remote)
	}

	return nil
}

func (f *forwarder) open(remote, local uint16, target string, shift bool) error {
	listeners, bound, err := listenLocal(local, shift)
	if err != nil {
		return err
	}

	t := &tunnel{remote: remote, local: bound, listeners: listeners, target: target}
	f.tunnels = append(f.tunnels, t)

	for _, listener := range listeners {
		go f.accept(listener, t)
	}

	note := ""
	if bound != local {
		note = fmt.Sprintf(" (port %d is in use on this Mac)", local)
	}

	_, _ = fmt.Fprintf(f.out, "localhost:%d -> VM port %d%s\n", bound, remote, note)

	return nil
}

func (f *forwarder) closeAll() {
	for _, t := range f.tunnels {
		t.close()
	}
}

func (f *forwarder) accept(listener net.Listener, t *tunnel) {
	for {
		connection, err := listener.Accept()
		if err != nil {
			return
		}

		go f.pipe(connection, t.destination())
	}
}

func (f *forwarder) pipe(local net.Conn, target string) {
	defer func() { _ = local.Close() }()

	remote, requests, err := f.openChannel(target)
	if err != nil {
		return
	}
	defer func() { _ = remote.Close() }()

	closed := make(chan struct{})

	go func() {
		ssh.DiscardRequests(requests)
		close(closed)
	}()

	toRemote, fromRemote := make(chan struct{}), make(chan struct{})

	go copyHalf(remote, local, toRemote)
	go copyHalf(local, remote, fromRemote)

	select {
	case <-toRemote:
		// The client has sent everything, and the service may still answer.
		<-fromRemote
	case <-fromRemote:
		// The service has sent everything. If it closed the channel too, nothing can reach it any more, and the client
		// is closed here instead of being waited for. With the end of the data only, the client may still send.
		select {
		case <-closed:
		case <-toRemote:
		}
	}
}

// openChannel opens the channel that ssh.Client.Dial opens, but keeps its requests, which end when the channel is
// closed. The net.Conn of Dial does not show that.
func (f *forwarder) openChannel(target string) (ssh.Channel, <-chan *ssh.Request, error) {
	host, portText, err := net.SplitHostPort(target)
	if err != nil {
		return nil, nil, err
	}

	port, err := strconv.ParseUint(portText, 10, 16)
	if err != nil {
		return nil, nil, err
	}

	return f.client.OpenChannel("direct-tcpip", ssh.Marshal(struct {
		Host       string
		Port       uint32
		OriginHost string
		OriginPort uint32
	}{Host: host, Port: uint32(port), OriginHost: "0.0.0.0"}))
}

func copyHalf(to io.Writer, from io.Reader, done chan<- struct{}) {
	_, _ = io.Copy(to, from)

	if half, ok := to.(interface{ CloseWrite() error }); ok {
		_ = half.CloseWrite()
	}

	close(done)
}

func listenLocal(port uint16, shift bool) ([]net.Listener, uint16, error) {
	last := int(port)
	if shift {
		last = min(last+spareLocalPorts, math.MaxUint16)
	}

	for candidate := int(port); candidate <= last; candidate++ {
		listeners, err := listenLoopback(uint16(candidate))
		if err == nil {
			return listeners, uint16(candidate), nil
		}

		if !errors.Is(err, syscall.EADDRINUSE) {
			return nil, 0, err
		}
	}

	if last == int(port) {
		return nil, 0, fmt.Errorf("port %d is in use on this Mac", port)
	}

	return nil, 0, fmt.Errorf("ports %d to %d are in use on this Mac", port, last)
}

func listenLoopback(port uint16) ([]net.Listener, error) {
	if inUse(port) {
		return nil, syscall.EADDRINUSE
	}

	v4, err := net.Listen("tcp4", net.JoinHostPort("127.0.0.1", strconv.Itoa(int(port))))
	if err != nil {
		return nil, err
	}

	v6, err := net.Listen("tcp6", net.JoinHostPort("::1", strconv.Itoa(int(port))))

	switch {
	case err == nil:
		return []net.Listener{v4, v6}, nil
	case errors.Is(err, syscall.EADDRINUSE):
		_ = v4.Close()
		return nil, err
	default:
		return []net.Listener{v4}, nil
	}
}
