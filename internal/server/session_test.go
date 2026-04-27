package server

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/binary"
	"encoding/pem"
	"io"
	"log"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/pizixi/gpipe/internal/codec"
	"github.com/pizixi/gpipe/internal/db"
	"github.com/pizixi/gpipe/internal/manager"
	"github.com/pizixi/gpipe/internal/model"
	"github.com/pizixi/gpipe/internal/pb"
	"github.com/pizixi/gpipe/internal/proxy"
)

func TestSessionClosesOnUnknownMessageID(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer clientConn.Close()

	logger := log.New(io.Discard, "", 0)
	session := &Session{
		id:         1,
		conn:       serverConn,
		hub:        NewHub(logger),
		logger:     logger,
		writeQueue: newWriteQueue(),
		closeCh:    make(chan struct{}),
	}

	done := make(chan struct{})
	go func() {
		session.Run()
		close(done)
	}()

	frame := make([]byte, 13)
	frame[0] = 33
	binary.BigEndian.PutUint32(frame[1:5], 8)
	binary.BigEndian.PutUint32(frame[5:9], ^uint32(0))
	binary.BigEndian.PutUint32(frame[9:13], 999999)
	if _, err := clientConn.Write(frame); err != nil {
		t.Fatalf("write bad frame: %v", err)
	}

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatalf("expected session to terminate after unknown message id")
	}
}

func TestWrapServerTLSConnTimesOutDuringHandshake(t *testing.T) {
	certFile, keyFile := writeTempCertificate(t)
	cfg, err := buildServerTLSConfig(certFile, keyFile)
	if err != nil {
		t.Fatalf("build server tls config: %v", err)
	}

	serverConn, clientConn := net.Pipe()
	defer clientConn.Close()

	oldTimeout := serverTLSHandshakeTimeout
	serverTLSHandshakeTimeout = 100 * time.Millisecond
	defer func() { serverTLSHandshakeTimeout = oldTimeout }()

	done := make(chan error, 1)
	go func() {
		_, err := wrapServerTLSConn(serverConn, cfg)
		done <- err
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatalf("expected server handshake timeout")
		}
	case <-time.After(time.Second):
		t.Fatalf("expected handshake timeout to complete")
	}
}

func TestSessionLoginUsesKeyOnly(t *testing.T) {
	database, err := db.Open("sqlite://file:test_session_login_by_key?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer database.Close()

	rt, err := manager.NewRuntime(database, nil)
	if err != nil {
		t.Fatalf("new runtime: %v", err)
	}

	_, _, playerID, err := rt.Players.Add("会话备注", "secretKey")
	if err != nil {
		t.Fatalf("add player: %v", err)
	}

	logger := log.New(io.Discard, "", 0)
	hub := NewHub(logger)
	hub.SetRuntime(rt)
	session := &Session{
		hub:        hub,
		logger:     logger,
		writeQueue: newWriteQueue(),
		closeCh:    make(chan struct{}),
	}

	reply := session.onLogin(&pb.LoginReq{
		Username: "ignored-user",
		Password: "secretKey",
	})
	ack, ok := reply.(*pb.LoginAck)
	if !ok {
		t.Fatalf("expected login ack, got %T", reply)
	}
	if ack.PlayerID != playerID {
		t.Fatalf("player id = %d, want %d", ack.PlayerID, playerID)
	}
	if session.playerID != playerID {
		t.Fatalf("session player id = %d, want %d", session.playerID, playerID)
	}
}

func TestPeerTunnelBroadcastsRuntimeEnabledOnLoginAndLogout(t *testing.T) {
	database, err := db.Open("sqlite://file:test_session_runtime_tunnel_state?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer database.Close()

	rt, err := manager.NewRuntime(database, nil)
	if err != nil {
		t.Fatalf("new runtime: %v", err)
	}

	_, _, receiverID, err := rt.Players.Add("receiver", "receiver-key")
	if err != nil {
		t.Fatalf("add receiver: %v", err)
	}
	_, _, senderID, err := rt.Players.Add("sender", "sender-key")
	if err != nil {
		t.Fatalf("add sender: %v", err)
	}
	if _, err := rt.Tunnel.Add(model.Tunnel{
		Source:           "127.0.0.1:" + freeTCPPort(t),
		Endpoint:         "127.0.0.1:9",
		Enabled:          true,
		Sender:           senderID,
		Receiver:         receiverID,
		TunnelType:       uint32(model.TunnelTypeTCP),
		EncryptionMethod: "None",
	}); err != nil {
		t.Fatalf("add tunnel: %v", err)
	}

	logger := log.New(io.Discard, "", 0)
	hub := NewHub(logger)
	hub.SetRuntime(rt)

	receiverSession, receiverPeer := newQueuedSession(hub, logger)
	defer receiverPeer.Close()
	defer receiverSession.Close()

	reply := receiverSession.onLogin(&pb.LoginReq{Password: "receiver-key"})
	ack, ok := reply.(*pb.LoginAck)
	if !ok {
		t.Fatalf("expected receiver login ack, got %T", reply)
	}
	if len(ack.TunnelList) != 1 {
		t.Fatalf("receiver tunnel count = %d, want %d", len(ack.TunnelList), 1)
	}
	if ack.TunnelList[0].Enabled {
		t.Fatalf("expected tunnel to stay disabled until sender is online")
	}

	senderSession, senderPeer := newQueuedSession(hub, logger)
	defer senderPeer.Close()
	reply = senderSession.onLogin(&pb.LoginReq{Password: "sender-key"})
	ack, ok = reply.(*pb.LoginAck)
	if !ok {
		t.Fatalf("expected sender login ack, got %T", reply)
	}
	if len(ack.TunnelList) != 1 || !ack.TunnelList[0].Enabled {
		t.Fatalf("expected sender login ack to carry enabled runtime tunnel, got %+v", ack.TunnelList)
	}

	onlineNtf, ok := popQueuedPush(t, receiverSession).(*pb.ModifyTunnelNtf)
	if !ok {
		t.Fatalf("expected receiver runtime update to be ModifyTunnelNtf")
	}
	if onlineNtf.IsDelete || onlineNtf.Tunnel == nil || !onlineNtf.Tunnel.Enabled {
		t.Fatalf("expected online runtime update to enable tunnel, got %+v", onlineNtf)
	}

	if err := senderSession.Close(); err != nil {
		t.Fatalf("close sender session: %v", err)
	}
	offlineNtf, ok := popQueuedPush(t, receiverSession).(*pb.ModifyTunnelNtf)
	if !ok {
		t.Fatalf("expected receiver offline update to be ModifyTunnelNtf")
	}
	if offlineNtf.IsDelete || offlineNtf.Tunnel == nil || offlineNtf.Tunnel.Enabled {
		t.Fatalf("expected offline runtime update to disable tunnel, got %+v", offlineNtf)
	}
}

func TestServerLocalTunnelFollowsPlayerOnlineState(t *testing.T) {
	database, err := db.Open("sqlite://file:test_server_local_tunnel_runtime_state?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer database.Close()

	rt, err := manager.NewRuntime(database, nil)
	if err != nil {
		t.Fatalf("new runtime: %v", err)
	}

	_, _, senderID, err := rt.Players.Add("server-local", "server-local-key")
	if err != nil {
		t.Fatalf("add sender: %v", err)
	}
	listenAddr := "127.0.0.1:" + freeTCPPort(t)
	if _, err := rt.Tunnel.Add(model.Tunnel{
		Source:           listenAddr,
		Endpoint:         "127.0.0.1:9",
		Enabled:          true,
		Sender:           senderID,
		Receiver:         0,
		TunnelType:       uint32(model.TunnelTypeTCP),
		EncryptionMethod: "None",
	}); err != nil {
		t.Fatalf("add tunnel: %v", err)
	}

	logger := log.New(io.Discard, "", 0)
	hub := NewHub(logger)
	hub.SetRuntime(rt)
	t.Cleanup(func() {
		if hub.proxyMgr != nil {
			hub.proxyMgr.Close()
		}
	})

	waitForTCPState(t, listenAddr, false)

	senderSession, senderPeer := newQueuedSession(hub, logger)
	defer senderPeer.Close()
	reply := senderSession.onLogin(&pb.LoginReq{Password: "server-local-key"})
	ack, ok := reply.(*pb.LoginAck)
	if !ok {
		t.Fatalf("expected login ack, got %T", reply)
	}
	if len(ack.TunnelList) != 1 || !ack.TunnelList[0].Enabled {
		t.Fatalf("expected server-local tunnel to enable after login, got %+v", ack.TunnelList)
	}

	waitForTCPState(t, listenAddr, true)

	if err := senderSession.Close(); err != nil {
		t.Fatalf("close sender session: %v", err)
	}
	waitForTCPState(t, listenAddr, false)
}

func TestSelfLoopTunnelStaysEnabledWhenOwnerLogsIn(t *testing.T) {
	database, err := db.Open("sqlite://file:test_session_self_loop_tunnel_runtime_state?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer database.Close()

	rt, err := manager.NewRuntime(database, nil)
	if err != nil {
		t.Fatalf("new runtime: %v", err)
	}

	_, _, playerID, err := rt.Players.Add("self-loop", "self-loop-key")
	if err != nil {
		t.Fatalf("add player: %v", err)
	}
	if _, err := rt.Tunnel.Add(model.Tunnel{
		Source:           "127.0.0.1:" + freeTCPPort(t),
		Endpoint:         "127.0.0.1:9",
		Enabled:          true,
		Sender:           playerID,
		Receiver:         playerID,
		TunnelType:       uint32(model.TunnelTypeTCP),
		EncryptionMethod: "None",
	}); err != nil {
		t.Fatalf("add tunnel: %v", err)
	}

	logger := log.New(io.Discard, "", 0)
	hub := NewHub(logger)
	hub.SetRuntime(rt)

	session, peer := newQueuedSession(hub, logger)
	defer peer.Close()
	defer session.Close()

	reply := session.onLogin(&pb.LoginReq{Password: "self-loop-key"})
	ack, ok := reply.(*pb.LoginAck)
	if !ok {
		t.Fatalf("expected login ack, got %T", reply)
	}
	if len(ack.TunnelList) != 1 || !ack.TunnelList[0].Enabled {
		t.Fatalf("expected self-loop tunnel to remain enabled, got %+v", ack.TunnelList)
	}
}

func TestTunnelRuntimeReportUpdatesClientInletStatus(t *testing.T) {
	database, err := db.Open("sqlite://file:test_session_client_tunnel_runtime_report?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer database.Close()

	rt, err := manager.NewRuntime(database, nil)
	if err != nil {
		t.Fatalf("new runtime: %v", err)
	}

	_, _, senderID, err := rt.Players.Add("sender", "sender-key")
	if err != nil {
		t.Fatalf("add sender: %v", err)
	}
	_, _, receiverID, err := rt.Players.Add("receiver", "receiver-key")
	if err != nil {
		t.Fatalf("add receiver: %v", err)
	}
	tunnel, err := rt.Tunnel.Add(model.Tunnel{
		Source:           "127.0.0.1:" + freeTCPPort(t),
		Endpoint:         "127.0.0.1:9",
		Enabled:          true,
		Sender:           senderID,
		Receiver:         receiverID,
		TunnelType:       uint32(model.TunnelTypeTCP),
		EncryptionMethod: "None",
	})
	if err != nil {
		t.Fatalf("add tunnel: %v", err)
	}

	logger := log.New(io.Discard, "", 0)
	hub := NewHub(logger)
	hub.SetRuntime(rt)

	session, peer := newQueuedSession(hub, logger)
	defer peer.Close()
	defer session.Close()

	reply := session.onLogin(&pb.LoginReq{Password: "receiver-key"})
	ack, ok := reply.(*pb.LoginAck)
	if !ok {
		t.Fatalf("expected login ack, got %T", reply)
	}
	if !ack.SupportsTunnelRuntimeReport {
		t.Fatalf("expected runtime report support flag")
	}

	session.handleTunnelRuntimeReport(&pb.TunnelRuntimeReport{
		TunnelID:  tunnel.ID,
		Component: string(proxy.RuntimeComponentInlet),
		Running:   false,
		Error:     "listen tcp 127.0.0.1:1234: bind: address already in use",
	})

	record, ok := rt.TunnelRuntime.Get(tunnel.ID)
	if !ok {
		t.Fatalf("expected runtime record")
	}
	if record.InletRunning {
		t.Fatalf("expected inlet running=false")
	}
	if record.InletError == "" {
		t.Fatalf("expected inlet error")
	}
}

func TestTunnelNotifierSendsSingleRuntimeUpdatePerPeer(t *testing.T) {
	database, err := db.Open("sqlite://file:test_session_tunnel_notifier_single_push?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer database.Close()

	logger := log.New(io.Discard, "", 0)
	hub := NewHub(logger)
	rt, err := manager.NewRuntime(database, hub)
	if err != nil {
		t.Fatalf("new runtime: %v", err)
	}
	hub.SetRuntime(rt)

	_, _, receiverID, err := rt.Players.Add("receiver", "receiver-key")
	if err != nil {
		t.Fatalf("add receiver: %v", err)
	}
	_, _, senderID, err := rt.Players.Add("sender", "sender-key")
	if err != nil {
		t.Fatalf("add sender: %v", err)
	}

	receiverSession, receiverPeer := newQueuedSession(hub, logger)
	defer receiverPeer.Close()
	defer receiverSession.Close()
	reply := receiverSession.onLogin(&pb.LoginReq{Password: "receiver-key"})
	if _, ok := reply.(*pb.LoginAck); !ok {
		t.Fatalf("expected receiver login ack, got %T", reply)
	}

	senderSession, senderPeer := newQueuedSession(hub, logger)
	defer senderPeer.Close()
	defer senderSession.Close()
	reply = senderSession.onLogin(&pb.LoginReq{Password: "sender-key"})
	if _, ok := reply.(*pb.LoginAck); !ok {
		t.Fatalf("expected sender login ack, got %T", reply)
	}

	if _, err := rt.Tunnel.Add(model.Tunnel{
		Source:           "127.0.0.1:" + freeTCPPort(t),
		Endpoint:         "127.0.0.1:9",
		Enabled:          true,
		Sender:           senderID,
		Receiver:         receiverID,
		TunnelType:       uint32(model.TunnelTypeTCP),
		EncryptionMethod: "None",
	}); err != nil {
		t.Fatalf("add tunnel: %v", err)
	}

	receiverNtf, ok := popQueuedPush(t, receiverSession).(*pb.ModifyTunnelNtf)
	if !ok {
		t.Fatalf("expected receiver tunnel update to be ModifyTunnelNtf")
	}
	if receiverNtf.Tunnel == nil || !receiverNtf.Tunnel.Enabled {
		t.Fatalf("expected receiver tunnel update to be enabled, got %+v", receiverNtf)
	}
	expectNoQueuedPushWithin(t, receiverSession, 200*time.Millisecond)

	senderNtf, ok := popQueuedPush(t, senderSession).(*pb.ModifyTunnelNtf)
	if !ok {
		t.Fatalf("expected sender tunnel update to be ModifyTunnelNtf")
	}
	if senderNtf.Tunnel == nil || !senderNtf.Tunnel.Enabled {
		t.Fatalf("expected sender tunnel update to be enabled, got %+v", senderNtf)
	}
	expectNoQueuedPushWithin(t, senderSession, 200*time.Millisecond)
}

func newQueuedSession(hub *Hub, logger *log.Logger) (*Session, net.Conn) {
	serverConn, clientConn := net.Pipe()
	return &Session{
		conn:       serverConn,
		hub:        hub,
		logger:     logger,
		writeQueue: newWriteQueue(),
		closeCh:    make(chan struct{}),
	}, clientConn
}

func popQueuedPush(t *testing.T, session *Session) any {
	t.Helper()

	type popResult struct {
		data []byte
		ok   bool
	}
	resultCh := make(chan popResult, 1)
	go func() {
		data, ok := session.writeQueue.Pop()
		resultCh <- popResult{data: data, ok: ok}
	}()

	select {
	case result := <-resultCh:
		if !result.ok {
			t.Fatalf("write queue closed before receiving push")
		}
		frame, rest, err := codec.TryExtractFrame(result.data, 2*1024*1024)
		if err != nil {
			t.Fatalf("extract pushed frame: %v", err)
		}
		if len(rest) != 0 {
			t.Fatalf("expected a single pushed frame, got trailing %d bytes", len(rest))
		}
		serial, message, err := codec.Decode(frame)
		if err != nil {
			t.Fatalf("decode pushed message: %v", err)
		}
		if serial != 0 {
			t.Fatalf("push serial = %d, want %d", serial, 0)
		}
		return message
	case <-time.After(time.Second):
		t.Fatalf("timeout waiting for queued push")
		return nil
	}
}

func expectNoQueuedPushWithin(t *testing.T, session *Session, wait time.Duration) {
	t.Helper()
	deadline := time.Now().Add(wait)
	for time.Now().Before(deadline) {
		session.writeQueue.mu.Lock()
		size := session.writeQueue.size
		closed := session.writeQueue.closed
		session.writeQueue.mu.Unlock()
		if size > 0 {
			t.Fatalf("unexpected extra queued push")
		}
		if closed {
			return
		}
		time.Sleep(10 * time.Millisecond)
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

func writeTempCertificate(t *testing.T) (string, string) {
	t.Helper()

	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate private key: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: "localhost",
		},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{"localhost"},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}

	privateKeyDER, err := x509.MarshalECPrivateKey(privateKey)
	if err != nil {
		t.Fatalf("marshal private key: %v", err)
	}

	dir := t.TempDir()
	certFile := filepath.Join(dir, "cert.pem")
	keyFile := filepath.Join(dir, "key.pem")

	if err := os.WriteFile(certFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER}), 0o600); err != nil {
		t.Fatalf("write cert file: %v", err)
	}
	if err := os.WriteFile(keyFile, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: privateKeyDER}), 0o600); err != nil {
		t.Fatalf("write key file: %v", err)
	}

	return certFile, keyFile
}
