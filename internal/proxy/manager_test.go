package proxy

import (
	"errors"
	"io"
	"log"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/pizixi/gpipe/internal/pb"
)

func TestSyncTunnelsRestartsInletWhenDescriptionChanges(t *testing.T) {
	logger := log.New(io.Discard, "", 0)
	manager := NewManager(logger, 0, func(playerID uint32, message any) error {
		_ = playerID
		_ = message
		return nil
	})

	port1 := freeTCPPort(t)
	port2 := freeTCPPort(t)

	tunnel1 := &pb.Tunnel{
		ID:               1,
		Enabled:          true,
		Sender:           1,
		Receiver:         0,
		TunnelType:       int32(pb.TunnelTypeTCP),
		Source:           &pb.TunnelPoint{Addr: "127.0.0.1:" + port1},
		Endpoint:         &pb.TunnelPoint{Addr: "127.0.0.1:9"},
		EncryptionMethod: "None",
	}
	manager.SyncTunnels([]*pb.Tunnel{tunnel1})
	waitForTCPState(t, "127.0.0.1:"+port1, true)

	tunnel2 := &pb.Tunnel{
		ID:               1,
		Enabled:          true,
		Sender:           1,
		Receiver:         0,
		TunnelType:       int32(pb.TunnelTypeTCP),
		Source:           &pb.TunnelPoint{Addr: "127.0.0.1:" + port2},
		Endpoint:         &pb.TunnelPoint{Addr: "127.0.0.1:9"},
		EncryptionMethod: "None",
	}
	manager.SyncTunnels([]*pb.Tunnel{tunnel2})
	waitForTCPState(t, "127.0.0.1:"+port1, false)
	waitForTCPState(t, "127.0.0.1:"+port2, true)

	manager.SyncTunnels(nil)
	waitForTCPState(t, "127.0.0.1:"+port2, false)
}

func TestUpdateTunnelTogglesInletRuntimeState(t *testing.T) {
	logger := log.New(io.Discard, "", 0)
	manager := NewManager(logger, 0, func(playerID uint32, message any) error {
		_ = playerID
		_ = message
		return nil
	})

	addr := "127.0.0.1:" + freeTCPPort(t)
	tunnel := &pb.Tunnel{
		ID:               2,
		Enabled:          true,
		Sender:           123,
		Receiver:         0,
		TunnelType:       int32(pb.TunnelTypeTCP),
		Source:           &pb.TunnelPoint{Addr: addr},
		Endpoint:         &pb.TunnelPoint{Addr: "127.0.0.1:9"},
		EncryptionMethod: "None",
	}
	manager.SyncTunnels([]*pb.Tunnel{tunnel})
	t.Cleanup(func() { manager.SyncTunnels(nil) })
	waitForTCPState(t, addr, true)

	disabled := *tunnel
	disabled.Enabled = false
	manager.UpdateTunnel(&pb.ModifyTunnelNtf{Tunnel: &disabled})
	waitForTCPState(t, addr, false)

	reEnabled := disabled
	reEnabled.Enabled = true
	manager.UpdateTunnel(&pb.ModifyTunnelNtf{Tunnel: &reEnabled})
	waitForTCPState(t, addr, true)

	manager.UpdateTunnel(&pb.ModifyTunnelNtf{IsDelete: true, Tunnel: &reEnabled})
	waitForTCPState(t, addr, false)
}

func TestUpdateTunnelStartsAndStopsInletByProtocol(t *testing.T) {
	cases := []struct {
		name     string
		mode     int32
		tcp      bool
		udp      bool
		password string
		method   string
	}{
		{name: "tcp", mode: int32(pb.TunnelTypeTCP), tcp: true, method: "None"},
		{name: "udp", mode: int32(pb.TunnelTypeUDP), udp: true, method: "None"},
		{name: "socks5", mode: int32(pb.TunnelTypeSOCKS5), tcp: true, method: "None"},
		{name: "http", mode: int32(pb.TunnelTypeHTTP), tcp: true, method: "None"},
		{name: "shadowsocks", mode: int32(pb.TunnelTypeShadowsocks), tcp: true, udp: true, password: "secret", method: DefaultShadowsocksMethod},
	}

	for idx, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			logger := log.New(io.Discard, "", 0)
			manager := NewManager(logger, 0, func(playerID uint32, message any) error {
				_ = playerID
				_ = message
				return nil
			})
			t.Cleanup(func() { manager.SyncTunnels(nil) })

			addr := "127.0.0.1:" + freeTCPUDPPort(t)
			tunnel := &pb.Tunnel{
				ID:               uint32(100 + idx),
				Enabled:          true,
				Sender:           123,
				Receiver:         0,
				TunnelType:       tc.mode,
				Source:           &pb.TunnelPoint{Addr: addr},
				Endpoint:         &pb.TunnelPoint{Addr: "127.0.0.1:9"},
				Password:         tc.password,
				EncryptionMethod: tc.method,
			}

			manager.SyncTunnels([]*pb.Tunnel{tunnel})
			if tc.tcp {
				waitForTCPState(t, addr, true)
			}
			if tc.udp {
				waitForUDPState(t, addr, true)
			}

			disabled := *tunnel
			disabled.Enabled = false
			manager.UpdateTunnel(&pb.ModifyTunnelNtf{Tunnel: &disabled})
			if tc.tcp {
				waitForTCPState(t, addr, false)
			}
			if tc.udp {
				waitForUDPState(t, addr, false)
			}
		})
	}
}

func TestManagerReportsServerInletStartFailure(t *testing.T) {
	logger := log.New(io.Discard, "", 0)
	manager := NewManager(logger, 0, func(playerID uint32, message any) error {
		_ = playerID
		_ = message
		return nil
	})

	events := make(chan TunnelRuntimeEvent, 4)
	manager.SetRuntimeReporter(func(event TunnelRuntimeEvent) {
		events <- event
	})

	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen occupied port: %v", err)
	}
	defer occupied.Close()

	tunnel := &pb.Tunnel{
		ID:               21,
		Enabled:          true,
		Sender:           123,
		Receiver:         0,
		TunnelType:       int32(pb.TunnelTypeTCP),
		Source:           &pb.TunnelPoint{Addr: occupied.Addr().String()},
		Endpoint:         &pb.TunnelPoint{Addr: "127.0.0.1:9"},
		EncryptionMethod: "None",
	}
	manager.SyncTunnels([]*pb.Tunnel{tunnel})
	t.Cleanup(func() { manager.SyncTunnels(nil) })

	event := waitForRuntimeEvent(t, events, RuntimeComponentInlet)
	if event.TunnelID != tunnel.ID {
		t.Fatalf("event tunnel id = %d, want %d", event.TunnelID, tunnel.ID)
	}
	if event.Running {
		t.Fatalf("expected inlet failure event, got running")
	}
	if event.Error == "" {
		t.Fatalf("expected inlet failure error")
	}
	if inlet := waitForInlet(t, manager, tunnel.ID); inlet != nil {
		t.Fatalf("expected failed inlet to stay uninstalled")
	}
}

func TestUpdateTunnelRestartsInletWhenDescriptionChanges(t *testing.T) {
	logger := log.New(io.Discard, "", 0)
	manager := NewManager(logger, 0, func(playerID uint32, message any) error {
		_ = playerID
		_ = message
		return nil
	})

	port1 := freeTCPPort(t)
	port2 := freeTCPPort(t)
	tunnel := &pb.Tunnel{
		ID:               3,
		Enabled:          true,
		Sender:           123,
		Receiver:         0,
		TunnelType:       int32(pb.TunnelTypeTCP),
		Source:           &pb.TunnelPoint{Addr: "127.0.0.1:" + port1},
		Endpoint:         &pb.TunnelPoint{Addr: "127.0.0.1:9"},
		EncryptionMethod: "None",
	}
	manager.SyncTunnels([]*pb.Tunnel{tunnel})
	t.Cleanup(func() { manager.SyncTunnels(nil) })
	waitForTCPState(t, "127.0.0.1:"+port1, true)

	updated := *tunnel
	updated.Source = &pb.TunnelPoint{Addr: "127.0.0.1:" + port2}
	manager.UpdateTunnel(&pb.ModifyTunnelNtf{Tunnel: &updated})
	waitForTCPState(t, "127.0.0.1:"+port1, false)
	waitForTCPState(t, "127.0.0.1:"+port2, true)
}

func TestUpdateTunnelReaffirmsRuntimeWhenDescriptionUnchanged(t *testing.T) {
	logger := log.New(io.Discard, "", 0)
	manager := NewManager(logger, 0, func(playerID uint32, message any) error {
		_ = playerID
		_ = message
		return nil
	})

	port := freeTCPPort(t)
	tunnel := &pb.Tunnel{
		ID:               31,
		Enabled:          true,
		Sender:           123,
		Receiver:         0,
		TunnelType:       int32(pb.TunnelTypeTCP),
		Source:           &pb.TunnelPoint{Addr: "127.0.0.1:" + port},
		Endpoint:         &pb.TunnelPoint{Addr: "127.0.0.1:9"},
		EncryptionMethod: "None",
	}
	manager.SyncTunnels([]*pb.Tunnel{tunnel})
	t.Cleanup(func() { manager.SyncTunnels(nil) })
	waitForTCPState(t, "127.0.0.1:"+port, true)

	// 仅在 SyncTunnels 完成后挂上 reporter，避免初始事件干扰断言。
	events := make(chan TunnelRuntimeEvent, 4)
	manager.SetRuntimeReporter(func(event TunnelRuntimeEvent) {
		events <- event
	})

	// 等同于服务端只更新了 Description 这类不影响 inlet 描述的字段。
	updated := *tunnel
	manager.UpdateTunnel(&pb.ModifyTunnelNtf{Tunnel: &updated})

	event := waitForRuntimeEvent(t, events, RuntimeComponentInlet)
	if event.TunnelID != tunnel.ID || !event.Running || event.Error != "" {
		t.Fatalf("expected inlet running re-affirm event, got %+v", event)
	}
}

func TestHandlePBRoutesByMessageDirection(t *testing.T) {
	logger := log.New(io.Discard, "", 0)

	serverManager := NewManager(logger, 0, func(playerID uint32, message any) error {
		_ = playerID
		_ = message
		return nil
	})
	serverTunnel := &pb.Tunnel{
		ID:               10,
		Enabled:          true,
		Sender:           123,
		Receiver:         0,
		TunnelType:       int32(pb.TunnelTypeTCP),
		Source:           &pb.TunnelPoint{Addr: "127.0.0.1:" + freeTCPPort(t)},
		Endpoint:         &pb.TunnelPoint{Addr: "127.0.0.1:9"},
		EncryptionMethod: "None",
	}
	serverManager.SyncTunnels([]*pb.Tunnel{serverTunnel})
	t.Cleanup(func() { serverManager.SyncTunnels(nil) })

	if inlet := waitForInlet(t, serverManager, serverTunnel.ID); inlet == nil {
		t.Fatalf("expected server inlet to exist")
	}

	conn, err := net.Dial("tcp", serverTunnel.Source.Addr)
	if err != nil {
		t.Fatalf("dial server inlet: %v", err)
	}
	defer conn.Close()

	var sessionID uint32
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		serverManager.inlets[serverTunnel.ID].mu.RLock()
		for id := range serverManager.inlets[serverTunnel.ID].sessions {
			sessionID = id
			break
		}
		serverManager.inlets[serverTunnel.ID].mu.RUnlock()
		if sessionID != 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if sessionID == 0 {
		t.Fatalf("expected inlet session to be created")
	}

	serverManager.HandlePB(&pb.O2IConnect{TunnelID: serverTunnel.ID, SessionID: sessionID, Success: true})
	deadline = time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if session := serverManager.inlets[serverTunnel.ID].session(sessionID); session != nil && session.context.ReadyForRead() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("expected O2IConnect to be delivered to server inlet")
}

func TestServerLocalTunnelFallsBackImmediatelyWhenRemotePlayerOffline(t *testing.T) {
	logger := log.New(io.Discard, "", 0)
	manager := NewManager(logger, 0, func(playerID uint32, message any) error {
		_ = playerID
		_ = message
		return errors.New("player offline")
	})

	tunnel := &pb.Tunnel{
		ID:               12,
		Enabled:          true,
		Sender:           123,
		Receiver:         0,
		TunnelType:       int32(pb.TunnelTypeTCP),
		Source:           &pb.TunnelPoint{Addr: "127.0.0.1:" + freeTCPPort(t)},
		Endpoint:         &pb.TunnelPoint{Addr: "127.0.0.1:9"},
		EncryptionMethod: "None",
	}
	manager.SyncTunnels([]*pb.Tunnel{tunnel})
	t.Cleanup(func() { manager.SyncTunnels(nil) })

	conn, err := net.Dial("tcp", tunnel.Source.Addr)
	if err != nil {
		t.Fatalf("dial server-local inlet: %v", err)
	}
	defer conn.Close()

	inlet := waitForInlet(t, manager, tunnel.ID)
	if inlet == nil {
		t.Fatalf("expected server-local inlet to exist")
	}

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		inlet.mu.RLock()
		sessionCount := len(inlet.sessions)
		inlet.mu.RUnlock()
		if sessionCount == 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("expected inlet session to be closed quickly after offline fallback")
}

func TestSyncTunnelsRestartsInletWhenCustomMappingChanges(t *testing.T) {
	logger := log.New(io.Discard, "", 0)
	manager := NewManager(logger, 0, func(playerID uint32, message any) error {
		_ = playerID
		_ = message
		return nil
	})

	tunnel1 := &pb.Tunnel{
		ID:               11,
		Enabled:          true,
		Sender:           1,
		Receiver:         0,
		TunnelType:       int32(pb.TunnelTypeSOCKS5),
		Source:           &pb.TunnelPoint{Addr: "127.0.0.1:" + freeTCPPort(t)},
		EncryptionMethod: "None",
		CustomMapping: map[string]string{
			"a.example.com": "127.0.0.1:80",
		},
	}
	manager.SyncTunnels([]*pb.Tunnel{tunnel1})
	t.Cleanup(func() { manager.SyncTunnels(nil) })

	inlet := waitForInlet(t, manager, tunnel1.ID)
	if inlet == nil {
		t.Fatalf("expected socks5 inlet to exist")
	}
	oldDescription := inlet.Description()

	tunnel2 := &pb.Tunnel{
		ID:               tunnel1.ID,
		Enabled:          true,
		Sender:           tunnel1.Sender,
		Receiver:         tunnel1.Receiver,
		TunnelType:       tunnel1.TunnelType,
		Source:           tunnel1.Source,
		EncryptionMethod: "None",
		CustomMapping: map[string]string{
			"a.example.com": "127.0.0.1:81",
		},
	}
	manager.SyncTunnels([]*pb.Tunnel{tunnel2})

	inlet = waitForInlet(t, manager, tunnel2.ID)
	if inlet == nil {
		t.Fatalf("expected updated inlet to exist")
	}
	newDescription := inlet.Description()
	if oldDescription == newDescription {
		t.Fatalf("expected inlet description to change after custom mapping update")
	}
	if newDescription != inletDescription(tunnel2) {
		t.Fatalf("expected inlet description to match latest tunnel config")
	}
}

func TestTunnelModeFromWireValue(t *testing.T) {
	if got := tunnelModeFromWireValue(3); got != TunnelModeHTTP {
		t.Fatalf("tunnelModeFromWireValue(3) = %v, want %v", got, TunnelModeHTTP)
	}
	if got := tunnelModeFromWireValue(4); got != TunnelModeShadowsocks {
		t.Fatalf("tunnelModeFromWireValue(4) = %v, want %v", got, TunnelModeShadowsocks)
	}
	if got := tunnelModeFromWireValue(99); got != TunnelModeUnknown {
		t.Fatalf("tunnelModeFromWireValue(99) = %v, want %v", got, TunnelModeUnknown)
	}
}

func TestUDPInletRecreatesSessionAfterIdleCleanup(t *testing.T) {
	logger := log.New(io.Discard, "", 0)
	inlet := NewInlet(
		logger,
		1,
		TunnelModeUDP,
		"127.0.0.1:"+freeTCPPort(t),
		"127.0.0.1:9",
		NewSessionCommonInfo(false, ParseEncryptionMethod("None"), nil),
		InletAuthData{},
		func(ProxyMessage) {},
		"udp-test",
	)
	if err := inlet.Start(); err != nil {
		t.Fatalf("start udp inlet: %v", err)
	}
	t.Cleanup(func() { _ = inlet.Stop() })

	addr, err := net.ResolveUDPAddr("udp", inlet.listenAddr)
	if err != nil {
		t.Fatalf("resolve udp addr: %v", err)
	}
	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		t.Fatalf("dial udp inlet: %v", err)
	}
	defer conn.Close()

	if _, err := conn.Write([]byte("hello")); err != nil {
		t.Fatalf("write first packet: %v", err)
	}

	firstSessionID := waitForUDPPeerSession(t, inlet, conn.LocalAddr().String())
	if firstSessionID == 0 {
		t.Fatalf("expected first udp session id")
	}

	inlet.closeSession(firstSessionID)
	if got := waitForUDPPeerSessionGone(t, inlet, conn.LocalAddr().String()); !got {
		t.Fatalf("expected udp peer mapping to be cleared after session close")
	}

	if _, err := conn.Write([]byte("world")); err != nil {
		t.Fatalf("write second packet: %v", err)
	}
	secondSessionID := waitForUDPPeerSession(t, inlet, conn.LocalAddr().String())
	if secondSessionID == 0 || secondSessionID == firstSessionID {
		t.Fatalf("expected udp session to be recreated, first=%d second=%d", firstSessionID, secondSessionID)
	}
}

func freeTCPPort(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("allocate port: %v", err)
	}
	defer ln.Close()
	return strconv.Itoa(int(ln.Addr().(*net.TCPAddr).AddrPort().Port()))
}

func freeTCPUDPPort(t *testing.T) string {
	t.Helper()
	for range 100 {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("allocate tcp port: %v", err)
		}
		port := strconv.Itoa(int(ln.Addr().(*net.TCPAddr).AddrPort().Port()))
		udpAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1:"+port)
		if err != nil {
			_ = ln.Close()
			t.Fatalf("resolve udp addr: %v", err)
		}
		udpConn, err := net.ListenUDP("udp", udpAddr)
		_ = ln.Close()
		if err != nil {
			continue
		}
		_ = udpConn.Close()
		return port
	}
	t.Fatalf("allocate shared tcp/udp port")
	return ""
}

func waitForTCPState(t *testing.T, addr string, wantListening bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		listening := err == nil
		if conn != nil {
			_ = conn.Close()
		}
		if listening == wantListening {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("tcp state mismatch for %s, want listening=%v", addr, wantListening)
}

func waitForUDPState(t *testing.T, addr string, wantListening bool) {
	t.Helper()
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		t.Fatalf("resolve udp addr: %v", err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.ListenUDP("udp", udpAddr)
		listening := err != nil
		if conn != nil {
			_ = conn.Close()
		}
		if listening == wantListening {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("udp state mismatch for %s, want listening=%v", addr, wantListening)
}

func waitForInlet(t *testing.T, manager *Manager, tunnelID uint32) *Inlet {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		manager.mu.RLock()
		inlet := manager.inlets[tunnelID]
		manager.mu.RUnlock()
		if inlet != nil {
			return inlet
		}
		time.Sleep(10 * time.Millisecond)
	}
	return nil
}

func waitForRuntimeEvent(t *testing.T, events <-chan TunnelRuntimeEvent, component RuntimeComponent) TunnelRuntimeEvent {
	t.Helper()
	deadline := time.After(time.Second)
	for {
		select {
		case event := <-events:
			if event.Component == component {
				return event
			}
		case <-deadline:
			t.Fatalf("timeout waiting for runtime event %s", component)
			return TunnelRuntimeEvent{}
		}
	}
}

func waitForUDPPeerSession(t *testing.T, inlet *Inlet, peer string) uint32 {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		inlet.mu.RLock()
		sessionID := inlet.udpPeers[peer]
		inlet.mu.RUnlock()
		if sessionID != 0 {
			return sessionID
		}
		time.Sleep(10 * time.Millisecond)
	}
	return 0
}

func waitForUDPPeerSessionGone(t *testing.T, inlet *Inlet, peer string) bool {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		inlet.mu.RLock()
		_, ok := inlet.udpPeers[peer]
		inlet.mu.RUnlock()
		if !ok {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
}
