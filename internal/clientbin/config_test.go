package clientbin

import "testing"

// 占位模板不应被误判为可运行的真实配置。
func TestPlaceholderDecodeReturnsEmptyConfig(t *testing.T) {
	got, err := Decode(PlaceholderValue())
	if err != nil {
		t.Fatalf("Decode(placeholder) error = %v", err)
	}
	if got.HasRequiredRuntimeValues() {
		t.Fatalf("placeholder decode should not produce runtime config: %+v", got)
	}
}

// 验证二进制内置配置的编码和解码过程保持一致。
func TestEncodeDecodeRoundTrip(t *testing.T) {
	want := EmbeddedConfig{
		Server:         "tcp://127.0.0.1:8118",
		Key:            "player-key",
		EnableTLS:      true,
		TLSServerName:  "demo.local",
		UseShadowsocks: true,
		SSServer:       "127.0.0.1:8388",
		SSMethod:       "aes-128-gcm",
		SSPassword:     "secret",
	}
	encoded, err := Encode(want)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	got, err := Decode(encoded)
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if got != want {
		t.Fatalf("round trip mismatch: got %+v want %+v", got, want)
	}
}
