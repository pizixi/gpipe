package manager

import (
	"testing"

	"github.com/pizixi/gpipe/internal/db"
	"github.com/pizixi/gpipe/internal/model"
	"github.com/pizixi/gpipe/internal/proxy"
)

type tunnelNotifierStub struct{}

func (t tunnelNotifierStub) BroadcastTunnel(playerID uint32, tunnel model.Tunnel, isDelete bool) {
	_ = playerID
	_ = tunnel
	_ = isDelete
}

func TestAddRejectsUnsupportedTunnelType(t *testing.T) {
	database, err := db.Open("sqlite://file:test_tunnel_manager?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer database.Close()

	rt, err := NewRuntime(database, tunnelNotifierStub{})
	if err != nil {
		t.Fatalf("new runtime: %v", err)
	}

	tunnel := model.Tunnel{
		Source:           "127.0.0.1:1080",
		Endpoint:         "127.0.0.1:9000",
		Enabled:          true,
		Sender:           0,
		Receiver:         0,
		TunnelType:       99,
		IsCompressed:     true,
		EncryptionMethod: "None",
	}
	if _, err := rt.Tunnel.Add(tunnel); err == nil {
		t.Fatalf("expected unsupported tunnel type to be rejected")
	}
}

func TestAddRejectsNonexistentSenderOrReceiverID(t *testing.T) {
	database, err := db.Open("sqlite://file:test_tunnel_manager_missing_player?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer database.Close()

	rt, err := NewRuntime(database, tunnelNotifierStub{})
	if err != nil {
		t.Fatalf("new runtime: %v", err)
	}

	if _, err := rt.Tunnel.Add(model.Tunnel{
		Source:           "127.0.0.1:1081",
		Endpoint:         "127.0.0.1:9001",
		Enabled:          true,
		Sender:           12345678,
		Receiver:         0,
		TunnelType:       uint32(model.TunnelTypeTCP),
		IsCompressed:     true,
		EncryptionMethod: "None",
	}); err == nil || err.Error() != "sender player does not exist" {
		t.Fatalf("expected missing sender id to be rejected, got err=%v", err)
	}

	if _, err := rt.Tunnel.Add(model.Tunnel{
		Source:           "127.0.0.1:1082",
		Endpoint:         "127.0.0.1:9002",
		Enabled:          true,
		Sender:           0,
		Receiver:         87654321,
		TunnelType:       uint32(model.TunnelTypeTCP),
		IsCompressed:     true,
		EncryptionMethod: "None",
	}); err == nil || err.Error() != "receiver player does not exist" {
		t.Fatalf("expected missing receiver id to be rejected, got err=%v", err)
	}
}

func TestUpdateRejectsNonexistentTunnelID(t *testing.T) {
	database, err := db.Open("sqlite://file:test_tunnel_manager_missing_tunnel?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer database.Close()

	rt, err := NewRuntime(database, tunnelNotifierStub{})
	if err != nil {
		t.Fatalf("new runtime: %v", err)
	}

	err = rt.Tunnel.Update(model.Tunnel{
		ID:               99999,
		Source:           "127.0.0.1:1083",
		Endpoint:         "127.0.0.1:9003",
		Enabled:          true,
		Sender:           0,
		Receiver:         0,
		TunnelType:       uint32(model.TunnelTypeTCP),
		IsCompressed:     true,
		EncryptionMethod: "None",
	})
	if err == nil || err.Error() != "tunnel id does not exist" {
		t.Fatalf("expected missing tunnel id to be rejected, got err=%v", err)
	}
}

func TestAddShadowsocksDefaultsMethodAndClearsUsername(t *testing.T) {
	database, err := db.Open("sqlite://file:test_tunnel_manager_shadowsocks?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer database.Close()

	rt, err := NewRuntime(database, tunnelNotifierStub{})
	if err != nil {
		t.Fatalf("new runtime: %v", err)
	}

	tunnel, err := rt.Tunnel.Add(model.Tunnel{
		Source:     "127.0.0.1:2080",
		Enabled:    true,
		Sender:     0,
		Receiver:   0,
		TunnelType: uint32(model.TunnelTypeShadowsocks),
		Password:   "secret",
		Username:   "should-clear",
	})
	if err != nil {
		t.Fatalf("add shadowsocks tunnel: %v", err)
	}

	if tunnel.EncryptionMethod != proxy.DefaultShadowsocksMethod {
		t.Fatalf("encryption_method = %q, want %q", tunnel.EncryptionMethod, proxy.DefaultShadowsocksMethod)
	}
	if tunnel.Username != "" {
		t.Fatalf("username = %q, want empty", tunnel.Username)
	}
}

func TestAddRejectsUnsupportedShadowsocksMethod(t *testing.T) {
	database, err := db.Open("sqlite://file:test_tunnel_manager_shadowsocks_invalid?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer database.Close()

	rt, err := NewRuntime(database, tunnelNotifierStub{})
	if err != nil {
		t.Fatalf("new runtime: %v", err)
	}

	if _, err := rt.Tunnel.Add(model.Tunnel{
		Source:           "127.0.0.1:3080",
		Enabled:          true,
		Sender:           0,
		Receiver:         0,
		TunnelType:       uint32(model.TunnelTypeShadowsocks),
		Password:         "secret",
		EncryptionMethod: "unsupported-method",
	}); err == nil {
		t.Fatalf("expected invalid shadowsocks method to be rejected")
	}
}

func TestQueryClampsNegativePageNumber(t *testing.T) {
	database, err := db.Open("sqlite://file:test_tunnel_manager_negative_page?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer database.Close()

	rt, err := NewRuntime(database, tunnelNotifierStub{})
	if err != nil {
		t.Fatalf("new runtime: %v", err)
	}

	for _, source := range []string{"127.0.0.1:4080", "127.0.0.1:4081"} {
		if _, err := rt.Tunnel.Add(model.Tunnel{
			Source:           source,
			Endpoint:         "127.0.0.1:9000",
			Enabled:          true,
			Sender:           0,
			Receiver:         0,
			TunnelType:       uint32(model.TunnelTypeTCP),
			IsCompressed:     true,
			EncryptionMethod: "None",
		}); err != nil {
			t.Fatalf("add tunnel %s: %v", source, err)
		}
	}

	list := rt.Tunnel.Query(-1, 1)
	if len(list) != 1 {
		t.Fatalf("len(list) = %d, want %d", len(list), 1)
	}
	if list[0].Source != "127.0.0.1:4080" {
		t.Fatalf("unexpected first item: %+v", list[0])
	}
}
