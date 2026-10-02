package listener

import (
	"fmt"
	"time"
)

// ListenerInfo represents a single TCP listening socket with its owning process details.
// It is the central data type — used by the scanner, the TUI table, and Glazed CLI output.
type ListenerInfo struct {
	PID           int32   `json:"pid"            glazed:"pid"`
	Name          string  `json:"name"           glazed:"name"`
	Cmdline       string  `json:"cmdline"        glazed:"cmdline"`
	Exe           string  `json:"exe"            glazed:"exe"`
	Cwd           string  `json:"cwd"            glazed:"cwd"`
	Username      string  `json:"username"       glazed:"username"`
	Port          uint32  `json:"port"           glazed:"port"`
	Address       string  `json:"address"        glazed:"address"`
	Protocol      string  `json:"protocol"       glazed:"protocol"`
	Uptime        string  `json:"uptime"         glazed:"uptime"`
	UptimeSeconds int64   `json:"uptime_seconds" glazed:"uptime_seconds"`
	CPUPercent    float64 `json:"cpu_percent"    glazed:"cpu_percent"`
	// CPUSampled reports whether CPUPercent was derived from two readings.
	// When false, CPUPercent is 0 only because no rate could be computed yet.
	CPUSampled bool   `json:"cpu_sampled"    glazed:"cpu_sampled"`
	RSSBytes   uint64 `json:"rss_bytes"      glazed:"rss_bytes"`
	RSSHuman   string `json:"rss_human"      glazed:"rss_human"`

	// Warnings lists fields that could not be read, with a short reason, so
	// callers can distinguish "denied"/"unsupported" from "genuinely empty".
	// It is omitted from structured output when there is nothing to report.
	Warnings []string `json:"warnings,omitempty" glazed:"warnings"`
}

// FormatUptime converts a create-time in Unix milliseconds to a human-readable
// duration string like "3h15m" or "45s".
func FormatUptime(createTimeMS int64) (string, int64) {
	if createTimeMS <= 0 {
		return "unknown", 0
	}

	createTime := time.Unix(createTimeMS/1000, (createTimeMS%1000)*1_000_000)
	duration := time.Since(createTime)
	seconds := int64(duration.Seconds())

	if seconds < 0 {
		return "0s", 0
	}

	switch {
	case seconds < 60:
		return fmt.Sprintf("%ds", seconds), seconds
	case seconds < 3600:
		m := seconds / 60
		s := seconds % 60
		if s > 0 {
			return fmt.Sprintf("%dm%ds", m, s), seconds
		}
		return fmt.Sprintf("%dm", m), seconds
	case seconds < 86400:
		h := seconds / 3600
		m := (seconds % 3600) / 60
		if m > 0 {
			return fmt.Sprintf("%dh%dm", h, m), seconds
		}
		return fmt.Sprintf("%dh", h), seconds
	default:
		d := seconds / 86400
		h := (seconds % 86400) / 3600
		if h > 0 {
			return fmt.Sprintf("%dd%dh", d, h), seconds
		}
		return fmt.Sprintf("%dd", d), seconds
	}
}

// FormatBytes converts a byte count to a human-readable string like "45.2 MB".
func FormatBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
