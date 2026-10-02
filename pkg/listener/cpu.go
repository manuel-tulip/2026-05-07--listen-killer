package listener

import (
	"runtime"
	"sync"
	"time"
)

type cpuSample struct {
	total float64
	at    time.Time
}

// CPUSampler converts cumulative per-process CPU counters into percentages
// across successive scans.
//
// A single instantaneous reading cannot yield a CPU rate: gopsutil's
// Process.Percent(0) therefore returns 0 on the first call for any freshly
// created *Process, which is exactly why the original scanner always reported
// CPU 0 (it built a new Process on every scan). By caching the previous
// cumulative reading per PID, the second and later scans produce a real rate
// without blocking the scan for a sampling interval.
type CPUSampler struct {
	mu      sync.Mutex
	samples map[int32]cpuSample
	numCPU  int
}

// NewCPUSampler returns an empty sampler sized for the host's CPU count.
func NewCPUSampler() *CPUSampler {
	return &CPUSampler{
		samples: make(map[int32]cpuSample),
		numCPU:  runtime.NumCPU(),
	}
}

// Observe records a cumulative CPU-seconds reading and returns the percentage
// since the previous observation. The first observation for a PID returns
// (0, false) because no rate can be derived from a single reading; subsequent
// observations return (percent, true). The formula matches gopsutil's Percent:
// 100% means one fully busy core, and a multithreaded process may exceed 100%.
func (c *CPUSampler) Observe(pid int32, totalSeconds float64, at time.Time) (float64, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	prev, ok := c.samples[pid]
	c.samples[pid] = cpuSample{total: totalSeconds, at: at}
	if !ok {
		return 0, false
	}

	wall := at.Sub(prev.at).Seconds()
	if wall <= 0 {
		return 0, true
	}
	delta := totalSeconds - prev.total
	if delta < 0 {
		// Counter went backwards (PID reuse); reset and report nothing rather
		// than a nonsensical negative value.
		c.samples[pid] = cpuSample{total: totalSeconds, at: at}
		return 0, false
	}
	return (delta / wall) * 100, true
}

// Prune drops cached samples for PIDs that were not observed in the latest
// scan, bounding memory in long-running TUI sessions.
func (c *CPUSampler) Prune(seen map[int32]bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for pid := range c.samples {
		if !seen[pid] {
			delete(c.samples, pid)
		}
	}
}
