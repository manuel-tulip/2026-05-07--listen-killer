//go:build !darwin

package listener

import (
	"fmt"
	"syscall"

	"github.com/shirou/gopsutil/v3/net"
)

// listSockets returns every TCP socket in the LISTEN state.
//
// On every platform except Darwin the kernel exposes the socket table directly
// (Linux reads /proc/net/tcp via gopsutil), so this is a single library call.
// See socket_source_darwin.go for the macOS implementation, which must shell
// out to lsof instead.
func listSockets() ([]RawSocket, error) {
	conns, err := net.Connections("inet")
	if err != nil {
		return nil, fmt.Errorf("failed to get network connections: %w", err)
	}

	var listeners []RawSocket
	for _, conn := range conns {
		// Filter: only TCP sockets in LISTEN state.
		if conn.Type != uint32(syscall.SOCK_STREAM) || conn.Status != "LISTEN" {
			continue
		}

		address := conn.Laddr.IP
		if address == "" {
			address = "*"
		}

		listeners = append(listeners, RawSocket{
			PID:     conn.Pid,
			Port:    conn.Laddr.Port,
			Address: address,
			Proto:   "TCP",
		})
	}

	return listeners, nil
}
