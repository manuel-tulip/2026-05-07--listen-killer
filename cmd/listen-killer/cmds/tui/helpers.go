package tui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/go-go-golems/glazed/pkg/types"

	"github.com/wesen/listen-killer/pkg/listener"
)

func normalizeAddress(addr string) string {
	if addr == "" || addr == "::" {
		return "*"
	}
	return addr
}

func formatFloat(f float64) string {
	if f == float64(int64(f)) {
		return fmt.Sprintf("%.0f", f)
	}
	return fmt.Sprintf("%.1f", f)
}

func listenerRow(l listener.ListenerInfo) types.Row {
	cpuStr := ""
	if l.CPUSampled {
		cpuStr = formatFloat(l.CPUPercent)
	}

	return types.NewRow(
		types.MRP("pid", l.PID),
		types.MRP("name", l.Name),
		types.MRP("cmdline", l.Cmdline),
		types.MRP("exe", l.Exe),
		types.MRP("cwd", l.Cwd),
		types.MRP("username", l.Username),
		types.MRP("port", l.Port),
		types.MRP("address", normalizeAddress(l.Address)),
		types.MRP("protocol", l.Protocol),
		types.MRP("uptime", l.Uptime),
		types.MRP("uptime_seconds", l.UptimeSeconds),
		types.MRP("cpu_percent", cpuStr),
		types.MRP("cpu_sampled", l.CPUSampled),
		types.MRP("rss_bytes", l.RSSBytes),
		types.MRP("rss_human", l.RSSHuman),
		types.MRP("warnings", strings.Join(l.Warnings, "; ")),
	)
}

func parseInt32List(values []string, fieldName string) (map[int32]bool, error) {
	ret := map[int32]bool{}
	for _, value := range expandStringList(values) {
		if value == "" {
			continue
		}
		n, err := strconv.ParseInt(value, 10, 32)
		if err != nil || n <= 0 {
			return nil, fmt.Errorf("invalid %s %q", fieldName, value)
		}
		ret[int32(n)] = true
	}
	return ret, nil
}

func parsePortList(values []string, fieldName string) (map[uint32]bool, error) {
	ret := map[uint32]bool{}
	for _, value := range expandStringList(values) {
		if value == "" {
			continue
		}
		value = strings.TrimPrefix(strings.TrimPrefix(value, ":"), "port:")
		n, err := strconv.ParseUint(value, 10, 32)
		if err != nil || n == 0 || n > 65535 {
			return nil, fmt.Errorf("invalid %s %q", fieldName, value)
		}
		ret[uint32(n)] = true
	}
	return ret, nil
}

func expandStringList(values []string) []string {
	var ret []string
	for _, value := range values {
		for _, part := range strings.Split(value, ",") {
			part = strings.TrimSpace(part)
			if part != "" {
				ret = append(ret, part)
			}
		}
	}
	return ret
}

func containsFold(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}
