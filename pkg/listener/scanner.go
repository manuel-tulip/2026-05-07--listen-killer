package listener

import (
	"sync"
	"time"
)

// Scanner discovers listening TCP sockets and enriches them with process
// metadata. It is portable: the OS-specific work lives behind listSockets()
// (build-tagged) and gatherFacts() (gopsutil).
//
// A Scanner retains per-process CPU samples across Scan calls, so callers that
// scan repeatedly (the TUI, which auto-refreshes) get a real cpu_percent from
// the second scan onward. A one-shot caller can use ScanWithSample to pay a
// short blocking interval and still get a value on the first result.
type Scanner struct {
	mu  sync.Mutex
	cpu *CPUSampler
}

// NewScanner creates a Scanner with an empty CPU sample cache.
func NewScanner() *Scanner {
	return &Scanner{cpu: NewCPUSampler()}
}

// ScanListeners performs a one-shot scan with no CPU warm-up. cpu_percent is
// empty because a single reading cannot establish a rate.
func ScanListeners() ([]ListenerInfo, error) {
	return NewScanner().Scan()
}

// Scan enumerates listeners and enriches them, reusing cached CPU samples.
func (s *Scanner) Scan() ([]ListenerInfo, error) {
	return s.scan()
}

// ScanWithSample performs a warm-up scan, waits interval, then scans again so
// the returned rows carry a real cpu_percent. The extra interval is paid
// deliberately and only when requested.
func (s *Scanner) ScanWithSample(interval time.Duration) ([]ListenerInfo, error) {
	if _, err := s.scan(); err != nil {
		return nil, err
	}
	if interval > 0 {
		time.Sleep(interval)
	}
	return s.scan()
}

func (s *Scanner) scan() ([]ListenerInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	socks, err := listSockets()
	if err != nil {
		return nil, err
	}

	now := time.Now()
	seen := make(map[int32]bool, len(socks))
	factsCache := make(map[int32]*ProcessFacts)
	cpuByPID := make(map[int32]float64)
	cpuSampled := make(map[int32]bool)
	var listeners []ListenerInfo

	for _, sock := range socks {
		facts, ok := factsCache[sock.PID]
		if !ok {
			facts, err = gatherFacts(sock.PID)
			if err != nil {
				// Skip processes we cannot identify (kernel-owned, gone,
				// permissions). Matches the original behavior.
				continue
			}
			factsCache[sock.PID] = facts
			// Observe once per PID: a process may own several listeners, and
			// observing again in the same instant would reset its sample.
			cpuByPID[sock.PID], cpuSampled[sock.PID] = s.cpu.Observe(sock.PID, facts.CPUTotal, now)
		}
		seen[sock.PID] = true

		address := sock.Address
		if address == "" {
			address = "*"
		}

		uptime, uptimeSecs := FormatUptime(facts.CreateMS)

		listeners = append(listeners, ListenerInfo{
			PID:           sock.PID,
			Name:          facts.Name,
			Cmdline:       facts.Cmdline,
			Exe:           facts.Exe,
			Cwd:           facts.Cwd,
			Username:      facts.Username,
			Port:          sock.Port,
			Address:       address,
			Protocol:      sock.Proto,
			Uptime:        uptime,
			UptimeSeconds: uptimeSecs,
			CPUPercent:    cpuByPID[sock.PID],
			CPUSampled:    cpuSampled[sock.PID],
			RSSBytes:      facts.RSS,
			RSSHuman:      FormatBytes(facts.RSS),
			Warnings:      facts.Warnings,
		})
	}

	s.cpu.Prune(seen)
	return listeners, nil
}
