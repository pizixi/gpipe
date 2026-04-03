package model

import "testing"

func TestNormalizeTunnelType(t *testing.T) {
	cases := []struct {
		value uint32
		want  TunnelType
	}{
		{value: 0, want: TunnelTypeTCP},
		{value: 1, want: TunnelTypeUDP},
		{value: 2, want: TunnelTypeSOCKS5},
		{value: 3, want: TunnelTypeHTTP},
		{value: 4, want: TunnelTypeShadowsocks},
		{value: 99, want: TunnelTypeUnknown},
	}

	for _, tc := range cases {
		if got := NormalizeTunnelType(tc.value); got != tc.want {
			t.Fatalf("NormalizeTunnelType(%d) = %v, want %v", tc.value, got, tc.want)
		}
	}
}

func TestInletDescriptionIncludesStableCustomMapping(t *testing.T) {
	tunnelA := Tunnel{
		ID:               1,
		Source:           "127.0.0.1:1000",
		Endpoint:         "127.0.0.1:2000",
		Sender:           1,
		Receiver:         2,
		TunnelType:       uint32(TunnelTypeHTTP),
		Username:         "u",
		Password:         "p",
		Enabled:          true,
		IsCompressed:     true,
		EncryptionMethod: "None",
		CustomMapping: map[string]string{
			"b.example.com": "127.0.0.1:81",
			"a.example.com": "127.0.0.1:80",
		},
	}
	tunnelB := tunnelA
	tunnelB.CustomMapping = map[string]string{
		"a.example.com": "127.0.0.1:80",
		"b.example.com": "127.0.0.1:81",
	}

	if tunnelA.InletDescription() != tunnelB.InletDescription() {
		t.Fatalf("expected inlet description to be stable across map iteration order")
	}

	tunnelC := tunnelA
	tunnelC.CustomMapping = map[string]string{
		"a.example.com": "127.0.0.1:80",
		"b.example.com": "127.0.0.1:82",
	}
	if tunnelA.InletDescription() == tunnelC.InletDescription() {
		t.Fatalf("expected inlet description to change when custom mapping changes")
	}
}
