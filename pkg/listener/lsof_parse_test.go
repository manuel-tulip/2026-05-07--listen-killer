package listener

import "testing"

func TestParseLsofListenLine(t *testing.T) {
	tests := []struct {
		name   string
		line   string
		want   RawSocket
		wantOK bool
	}{
		{
			name:   "ipv4 loopback listener",
			line:   "com.docke  6405 manuel 5u IPv4 0xa4ac69c8e81c7a22 0t0 TCP 127.0.0.1:5432 (LISTEN)",
			want:   RawSocket{PID: 6405, Port: 5432, Address: "127.0.0.1", Proto: "TCP"},
			wantOK: true,
		},
		{
			name:   "wildcard ipv6 listener",
			line:   "node       3142 manuel 53u IPv6 0x72aef3ef7f6eae2a 0t0 TCP *:5184 (LISTEN)",
			want:   RawSocket{PID: 3142, Port: 5184, Address: "*", Proto: "TCP"},
			wantOK: true,
		},
		{
			name:   "ipv6 loopback listener",
			line:   "node       9726 manuel 60u IPv6 0xb1b721780fd380c2 0t0 TCP [::1]:5199 (LISTEN)",
			want:   RawSocket{PID: 9726, Port: 5199, Address: "::1", Proto: "TCP"},
			wantOK: true,
		},
		{
			name:   "header row",
			line:   "COMMAND     PID USER   FD   TYPE DEVICE SIZE/OFF NODE NAME",
			wantOK: false,
		},
		{
			name:   "established connection is not a listener",
			line:   "node       3142 manuel 57u IPv6 0x6ab92c4de7faeb21 0t0 TCP [::1]:5184->[::1]:64806 (ESTABLISHED)",
			wantOK: false,
		},
		{
			name:   "udp row",
			line:   "mDNSRespo   812 manuel  7u IPv4 0xabc 0t0 UDP *:5353",
			wantOK: false,
		},
		{
			name:   "too few fields",
			line:   "node 3142 manuel 53u IPv6 0xabc",
			wantOK: false,
		},
		{
			name:   "non numeric pid",
			line:   "node       xxxx manuel 53u IPv6 0xabc 0t0 TCP *:5184 (LISTEN)",
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseLsofListenLine(tt.line)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v (line=%q)", ok, tt.wantOK, tt.line)
			}
			if ok && got != tt.want {
				t.Fatalf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}
