package forward

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net/netip"
	"strconv"
	"strings"
)

const listenState = "0A"

type socket struct {
	address netip.Addr
	port    uint16
}

func (s socket) target() string {
	address := s.address.Unmap()

	switch {
	case address.IsUnspecified() && address.Is4():
		address = netip.AddrFrom4([4]byte{127, 0, 0, 1})
	case address.IsUnspecified():
		address = netip.IPv6Loopback()
	}

	return netip.AddrPortFrom(address, s.port).String()
}

func parseSockets(table string) ([]socket, error) {
	var sockets []socket

	for line := range strings.Lines(table) {
		fields := strings.Fields(line)
		if len(fields) < 4 || fields[3] != listenState {
			continue
		}

		address, port, err := parseEndpoint(fields[1])
		if err != nil {
			return nil, err
		}

		sockets = append(sockets, socket{address: address, port: port})
	}

	return sockets, nil
}

func parseEndpoint(field string) (netip.Addr, uint16, error) {
	invalid := fmt.Errorf("unexpected socket table address %q", field)

	host, port, found := strings.Cut(field, ":")
	if !found {
		return netip.Addr{}, 0, invalid
	}

	number, err := strconv.ParseUint(port, 16, 16)
	if err != nil {
		return netip.Addr{}, 0, invalid
	}

	address, err := parseAddress(host)
	if err != nil {
		return netip.Addr{}, 0, invalid
	}

	return address, uint16(number), nil
}

func parseAddress(host string) (netip.Addr, error) {
	raw := make([]byte, 0, len(host)/2)

	for ; len(host) >= 8; host = host[8:] {
		word, err := strconv.ParseUint(host[:8], 16, 32)
		if err != nil {
			return netip.Addr{}, err
		}

		raw = binary.LittleEndian.AppendUint32(raw, uint32(word))
	}

	address, ok := netip.AddrFromSlice(raw)
	if !ok || host != "" {
		return netip.Addr{}, errors.New("not an address")
	}

	return address, nil
}
