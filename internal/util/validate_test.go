package util

import "testing"

func TestIsValidTunnelSourceAddress(t *testing.T) {
	cases := []struct {
		addr string
		want bool
	}{
		{addr: "127.0.0.1:1080", want: true},
		{addr: ":1080", want: true},
		{addr: "localhost:1080", want: true},
		{addr: " example.com:1080 ", want: true},
		{addr: "bad host:1080", want: false},
		{addr: "127.0.0.1", want: false},
	}

	for _, tc := range cases {
		if got := IsValidTunnelSourceAddress(tc.addr); got != tc.want {
			t.Fatalf("IsValidTunnelSourceAddress(%q) = %v, want %v", tc.addr, got, tc.want)
		}
	}
}

func TestIsValidTunnelEndpointAddress(t *testing.T) {
	cases := []struct {
		addr string
		want bool
	}{
		{addr: "127.0.0.1:9000", want: true},
		{addr: "example.com:9000", want: true},
		{addr: "localhost:9000", want: true},
		{addr: ":9000", want: false},
		{addr: "bad host:9000", want: false},
	}

	for _, tc := range cases {
		if got := IsValidTunnelEndpointAddress(tc.addr); got != tc.want {
			t.Fatalf("IsValidTunnelEndpointAddress(%q) = %v, want %v", tc.addr, got, tc.want)
		}
	}
}

func TestGetTunnelPort(t *testing.T) {
	port, ok := GetTunnelPort(" localhost:8080 ")
	if !ok || port != 8080 {
		t.Fatalf("GetTunnelPort returned (%d, %v), want (8080, true)", port, ok)
	}
}
