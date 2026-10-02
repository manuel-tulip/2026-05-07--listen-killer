package listener

import (
	"fmt"
	"strings"
	"syscall"
)

// KillProcess sends a signal to a process by PID.
// Valid signals: TERM, KILL, INT (case-insensitive).
func KillProcess(pid int32, signal string) error {
	var sig syscall.Signal
	switch strings.ToUpper(signal) {
	case "TERM":
		sig = syscall.SIGTERM
	case "KILL":
		sig = syscall.SIGKILL
	case "INT":
		sig = syscall.SIGINT
	default:
		return fmt.Errorf("unknown signal: %s (use TERM, KILL, or INT)", signal)
	}

	if err := syscall.Kill(int(pid), sig); err != nil {
		return fmt.Errorf("kill PID %d with %s: %w", pid, signal, err)
	}
	return nil
}
