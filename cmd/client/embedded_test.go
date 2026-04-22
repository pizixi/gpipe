package main

import (
	"testing"

	"github.com/pizixi/gpipe/internal/clientbin"
)

// 验证专属客户端在没有命令行参数时会默认执行 run。
func TestParseCommandFallsBackToRunWhenEmbeddedConfigExists(t *testing.T) {
	previous := embeddedClientConfig
	t.Cleanup(func() {
		embeddedClientConfig = previous
	})

	value, err := clientbin.Encode(clientbin.EmbeddedConfig{
		Server: "tcp://127.0.0.1:8118",
		Key:    "demo-key",
	})
	if err != nil {
		t.Fatalf("encode embedded config: %v", err)
	}
	embeddedClientConfig = value

	command, args, err := parseCommand(nil)
	if err != nil {
		t.Fatalf("parseCommand() error = %v", err)
	}
	if command != "run" {
		t.Fatalf("command = %q, want %q", command, "run")
	}
	if len(args) != 0 {
		t.Fatalf("args length = %d, want 0", len(args))
	}
}

// 验证内置配置可以作为默认值，同时仍允许命令行覆写。
func TestParseCommonArgsUsesEmbeddedDefaultsAndAllowsOverrides(t *testing.T) {
	previous := embeddedClientConfig
	t.Cleanup(func() {
		embeddedClientConfig = previous
	})

	value, err := clientbin.Encode(clientbin.EmbeddedConfig{
		Server:         "tcp://127.0.0.1:8118",
		Key:            "demo-key",
		EnableTLS:      true,
		TLSServerName:  "demo.local",
		UseShadowsocks: true,
		SSServer:       "127.0.0.1:8388",
		SSMethod:       "aes-128-gcm",
		SSPassword:     "ss-secret",
	})
	if err != nil {
		t.Fatalf("encode embedded config: %v", err)
	}
	embeddedClientConfig = value

	common, err := parseCommonArgs(nil)
	if err != nil {
		t.Fatalf("parseCommonArgs() error = %v", err)
	}
	if common.Server != "tcp://127.0.0.1:8118" || common.Key != "demo-key" {
		t.Fatalf("unexpected embedded defaults: %+v", common)
	}
	if !common.EnableTLS || common.SSServer != "127.0.0.1:8388" {
		t.Fatalf("expected embedded TLS and Shadowsocks defaults: %+v", common)
	}
	if common.LogDir != "" {
		t.Fatalf("embedded log dir = %q, want empty default", common.LogDir)
	}

	override, err := parseCommonArgs([]string{"--server=ws://127.0.0.1:8119"})
	if err != nil {
		t.Fatalf("parseCommonArgs override error = %v", err)
	}
	if override.Server != "ws://127.0.0.1:8119" {
		t.Fatalf("override server = %q, want %q", override.Server, "ws://127.0.0.1:8119")
	}
	if override.Key != "demo-key" {
		t.Fatalf("override key = %q, want embedded key", override.Key)
	}
}
