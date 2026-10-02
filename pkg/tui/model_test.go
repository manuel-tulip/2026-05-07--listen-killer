package tui

import (
	"testing"

	"github.com/wesen/listen-killer/pkg/listener"
)

func TestListenerURL(t *testing.T) {
	tests := []struct {
		address string
		port    uint32
		want    string
	}{
		{"127.0.0.1", 5432, "http://127.0.0.1:5432"},
		{"0.0.0.0", 8080, "http://127.0.0.1:8080"},
		{"*", 5184, "http://127.0.0.1:5184"},
		{"", 3000, "http://127.0.0.1:3000"},
		{"::", 3000, "http://127.0.0.1:3000"},
		{"::1", 5199, "http://[::1]:5199"},
		{"fe80::1", 4000, "http://[fe80::1]:4000"},
		{"192.168.1.10", 9000, "http://192.168.1.10:9000"},
	}

	for _, tt := range tests {
		t.Run(tt.address, func(t *testing.T) {
			got := listenerURL(&listener.ListenerInfo{Address: tt.address, Port: tt.port})
			if got != tt.want {
				t.Fatalf("listenerURL(%q:%d) = %q, want %q", tt.address, tt.port, got, tt.want)
			}
		})
	}
}
