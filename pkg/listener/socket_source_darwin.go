//go:build darwin

package listener

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// lsofCandidates are checked in order. macOS ships lsof at /usr/sbin/lsof,
// which is not always present on a minimal PATH, so we prefer the absolute
// path before falling back to PATH lookup.
var lsofCandidates = []string{"/usr/sbin/lsof", "/usr/local/sbin/lsof"}

const lsofTimeout = 15 * time.Second

// resolveLsof returns a usable lsof path or an actionable error. macOS has no
// /proc, so socket enumeration fundamentally depends on lsof (or an
// equivalent priv(1)/sysctl implementation).
func resolveLsof() (string, error) {
	for _, candidate := range lsofCandidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, nil
		}
	}
	if path, err := exec.LookPath("lsof"); err == nil {
		return path, nil
	}
	return "", errors.New("lsof not found: macOS socket discovery requires lsof " +
		"(expected /usr/sbin/lsof). Install the Xcode Command Line Tools or " +
		"run from an environment whose PATH includes /usr/sbin")
}

// listSockets enumerates listening TCP sockets on macOS by shelling out to
// lsof. Unlike Linux there is no kernel socket table we can read directly, so
// gopsutil uses the same strategy. We invoke lsof ourselves to (a) pin the
// binary path, (b) request only TCP (UDP can never be a listener we display,
// so this halves the work), and (c) turn failures into actionable errors.
func listSockets() ([]RawSocket, error) {
	lsofPath, err := resolveLsof()
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), lsofTimeout)
	defer cancel()

	// -a ANDs the selections, -n suppresses DNS, -P suppresses port names.
	cmd := exec.CommandContext(ctx, lsofPath, "-a", "-n", "-P", "-i", "tcp")
	out, err := cmd.Output()
	if err != nil {
		// lsof exits 1 when it finds nothing, and also when it could not
		// inspect some processes (e.g. permission denied). gopsutil treats an
		// empty result as "no connections", so mirror that: only fail when we
		// have no output at all.
		if len(out) == 0 {
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) && len(exitErr.Stderr) > 0 {
				return nil, fmt.Errorf("lsof failed: %w: %s", err, strings.TrimSpace(string(exitErr.Stderr)))
			}
			if errors.Is(err, context.DeadlineExceeded) {
				return nil, fmt.Errorf("lsof timed out after %s", lsofTimeout)
			}
			return nil, fmt.Errorf("running %s: %w", lsofPath, err)
		}
	}

	var listeners []RawSocket
	for _, line := range strings.Split(string(out), "\n") {
		if sock, ok := parseLsofListenLine(line); ok {
			listeners = append(listeners, sock)
		}
	}

	return listeners, nil
}
