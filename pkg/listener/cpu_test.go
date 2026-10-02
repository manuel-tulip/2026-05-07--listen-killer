package listener

import (
	"testing"
	"time"
)

func TestCPUSampler(t *testing.T) {
	c := NewCPUSampler()
	base := time.Unix(1000, 0)

	// First observation establishes a baseline and reports nothing sampled.
	if got, ok := c.Observe(42, 10.0, base); got != 0 || ok {
		t.Fatalf("first observe = (%v, %v), want (0, false)", got, ok)
	}

	// 0.5 CPU-seconds over 1 wall-second => 50%.
	if got, ok := c.Observe(42, 10.5, base.Add(time.Second)); !ok || got < 49.9 || got > 50.1 {
		t.Fatalf("second observe = (%v, %v), want (~50, true)", got, ok)
	}

	// Second PID is tracked independently.
	if got, ok := c.Observe(7, 1.0, base); got != 0 || ok {
		t.Fatalf("first observe pid 7 = (%v, %v), want (0, false)", got, ok)
	}
	if got, ok := c.Observe(7, 3.0, base.Add(2*time.Second)); !ok || got < 99.9 || got > 100.1 {
		t.Fatalf("pid 7 = (%v, %v), want (~100, true)", got, ok)
	}
}

func TestCPUSamplerZeroWallAndNegativeDelta(t *testing.T) {
	c := NewCPUSampler()
	base := time.Unix(2000, 0)
	c.Observe(1, 5.0, base)

	if got, ok := c.Observe(1, 6.0, base); got != 0 || !ok {
		t.Fatalf("zero wall = (%v, %v), want (0, true)", got, ok)
	}

	// Counter moved backwards (PID reuse) => not sampled, not negative.
	if got, ok := c.Observe(1, 1.0, base.Add(time.Second)); got != 0 || ok {
		t.Fatalf("negative delta = (%v, %v), want (0, false)", got, ok)
	}
	// Sample is reset, so the next forward delta works again.
	if got, ok := c.Observe(1, 1.5, base.Add(2*time.Second)); !ok || got < 49.9 || got > 50.1 {
		t.Fatalf("after reset = (%v, %v), want (~50, true)", got, ok)
	}
}

func TestCPUSamplerPrune(t *testing.T) {
	c := NewCPUSampler()
	base := time.Unix(3000, 0)
	c.Observe(1, 1.0, base)
	c.Observe(2, 1.0, base)
	c.Observe(2, 2.0, base.Add(time.Second))

	c.Prune(map[int32]bool{2: true})

	if _, ok := c.samples[1]; ok {
		t.Fatal("pid 1 should have been pruned")
	}
	if _, ok := c.samples[2]; !ok {
		t.Fatal("pid 2 should have been kept")
	}
}
