package listener

import (
	"net"
	"strconv"
	"strings"
)

// parseLsofListenLine parses one `lsof -a -n -P -i tcp` output line and reports
// whether it describes a TCP socket in the LISTEN state.
//
// A representative listener line (10 whitespace-separated fields):
//
//	node  3142 manuel.odendahl 53u IPv6 0x72ae... 0t0 TCP *:5184 (LISTEN)
//	node  9726 manuel.odendahl 60u IPv6 0xb1b7... 0t0 TCP [::1]:5199 (LISTEN)
//	com.d 6405 manuel.odendahl  5u IPv4 0xa4ac... 0t0 TCP 127.0.0.1:5432 (LISTEN)
//
// Field layout is fixed: 0=COMMAND 1=PID 2=USER 3=FD 4=TYPE 5=DEVICE
// 6=SIZE/OFF 7=NODE 8=NAME 9=(STATE). We only accept NODE==TCP and a trailing
// (LISTEN) state, which excludes UDP rows and established connections.
//
// This function is deliberately free of build tags so it can be unit-tested on
// any platform against captured fixtures.
func parseLsofListenLine(line string) (RawSocket, bool) {
	f := strings.Fields(line)
	if len(f) < 8 || f[0] == "COMMAND" {
		return RawSocket{}, false
	}
	if f[7] != "TCP" {
		return RawSocket{}, false
	}
	if len(f) != 10 || strings.Trim(f[9], "()") != "LISTEN" {
		return RawSocket{}, false
	}

	pid, err := strconv.ParseInt(f[1], 10, 32)
	if err != nil || pid <= 0 {
		return RawSocket{}, false
	}

	// NAME is "host:port". lsof prints IPv4/IPv6/wildcard hosts as
	// "127.0.0.1:5432", "[::1]:5199", or "*:5184".
	host, portStr, err := net.SplitHostPort(f[8])
	if err != nil {
		return RawSocket{}, false
	}
	port, err := strconv.ParseUint(portStr, 10, 32)
	if err != nil || port == 0 || port > 65535 {
		return RawSocket{}, false
	}
	if host == "" {
		host = "*"
	}

	return RawSocket{
		PID:     int32(pid),
		Port:    uint32(port),
		Address: host,
		Proto:   "TCP",
	}, true
}
