package listener

import (
	"errors"
	"os"
	"strings"

	"github.com/shirou/gopsutil/v3/process"
)

// ProcessFacts is the platform-neutral result of enriching a PID with process
// metadata. It is intentionally plain Go data so Scanner (and its tests) never
// depend on OS-specific structures.
type ProcessFacts struct {
	Name     string
	Cmdline  string
	Exe      string
	Cwd      string
	Username string
	RSS      uint64
	CreateMS int64
	// CPUTotal is cumulative user+system CPU time in seconds (a monotonic
	// counter, not a percentage). CPUSampler converts deltas into percents.
	CPUTotal float64
	// Warnings records fields that could not be read, and why, so callers can
	// distinguish "denied"/"unsupported" from "genuinely empty" instead of
	// silently discarding the error.
	Warnings []string
}

// gatherFacts reads process metadata via gopsutil. It deliberately does not
// fail when individual fields fail; instead it records a warning. Only a
// failure to identify the process at all is fatal.
func gatherFacts(pid int32) (*ProcessFacts, error) {
	p, err := process.NewProcess(pid)
	if err != nil {
		return nil, err
	}

	f := &ProcessFacts{}
	f.Name, _ = p.Name()
	f.Cmdline, _ = p.Cmdline()
	f.Username, _ = p.Username()
	f.CreateMS, _ = p.CreateTime()

	if exe, err := p.Exe(); err == nil {
		f.Exe = exe
	} else {
		f.Warnings = appendWarning(f.Warnings, "exe", err)
	}

	if cwd, err := p.Cwd(); err == nil {
		f.Cwd = cwd
	} else {
		f.Warnings = appendWarning(f.Warnings, "cwd", err)
	}

	if mem, err := p.MemoryInfo(); err == nil && mem != nil {
		f.RSS = mem.RSS
	}

	if ts, err := p.Times(); err == nil && ts != nil {
		f.CPUTotal = ts.User + ts.System
	}

	return f, nil
}

// appendWarning classifies an enrichment error into a short, user-facing
// warning. gopsutil's "not implemented" sentinel lives in an internal package
// we cannot import, so it is matched by message; the permission sentinel is
// exported and matched structurally.
func appendWarning(warnings []string, field string, err error) []string {
	if err == nil {
		return warnings
	}
	var reason string
	switch {
	case errors.Is(err, process.ErrorNotPermitted), errors.Is(err, os.ErrPermission):
		reason = "permission denied (grant Full Disk Access?)"
	case strings.Contains(err.Error(), "not implemented"):
		reason = "unsupported in this build (needs CGO_ENABLED=1)"
	default:
		reason = "unavailable"
	}
	return append(warnings, field+": "+reason)
}
