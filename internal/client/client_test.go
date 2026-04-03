package client

import (
	"context"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/pizixi/gpipe/internal/pb"
	"github.com/pizixi/gpipe/internal/proxy"
	"github.com/pizixi/gpipe/internal/transport"
)

func TestClientSessionCloseStopsProxyManager(t *testing.T) {
	logger := log.New(io.Discard, "", 0)
	session := newClientSession(Options{Logger: logger}, nil)
	session.playerID = 100
	session.proxyMgr = proxy.NewManager(logger, session.playerID, func(playerID uint32, message any) error {
		_ = playerID
		_ = message
		return nil
	})

	addr := "127.0.0.1:" + freeTCPPort(t)
	tunnel := &pb.Tunnel{
		ID:               1,
		Enabled:          true,
		Sender:           1,
		Receiver:         session.playerID,
		TunnelType:       int32(pb.TunnelTypeTCP),
		Source:           &pb.TunnelPoint{Addr: addr},
		Endpoint:         &pb.TunnelPoint{Addr: "127.0.0.1:9"},
		EncryptionMethod: "None",
	}

	session.proxyMgr.SyncTunnels([]*pb.Tunnel{tunnel})
	waitForTCPState(t, addr, true)

	session.close()
	waitForTCPState(t, addr, false)
}

func TestTLSConfigSkipsVerificationForEncryptionOnlyMode(t *testing.T) {
	app := New(Options{
		EnableTLS: true,
		CACert:    "../../certs/does-not-need-to-exist.pem",
		Logger:    log.New(io.Discard, "", 0),
	})
	u, err := url.Parse("tcp://localhost:443")
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}

	cfg, err := app.tlsConfig(u)
	if err != nil {
		t.Fatalf("tls config: %v", err)
	}
	if !cfg.InsecureSkipVerify {
		t.Fatalf("expected tls config to skip certificate verification")
	}
	if cfg.ServerName != "localhost" {
		t.Fatalf("server name = %q, want %q", cfg.ServerName, "localhost")
	}
}

func TestConnectTCPWithTLSTimesOutDuringHandshake(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen tcp: %v", err)
	}
	defer ln.Close()

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		time.Sleep(300 * time.Millisecond)
	}()

	oldTimeout := tlsHandshakeTimeout
	tlsHandshakeTimeout = 100 * time.Millisecond
	defer func() { tlsHandshakeTimeout = oldTimeout }()

	app := New(Options{
		EnableTLS: true,
		Insecure:  true,
		Logger:    log.New(io.Discard, "", 0),
	})
	u, err := url.Parse("tcp://" + ln.Addr().String())
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}

	start := time.Now()
	conn, err := app.connectTCP(u)
	if err == nil {
		if conn != nil {
			_ = conn.Close()
		}
		t.Fatalf("expected tls handshake timeout")
	}
	if elapsed := time.Since(start); elapsed >= 250*time.Millisecond {
		t.Fatalf("expected handshake timeout to fail fast, elapsed=%v", elapsed)
	}
}

func TestConnectTCPUsesCustomDial(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen tcp: %v", err)
	}
	defer ln.Close()

	accepted := make(chan struct{}, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		accepted <- struct{}{}
		_ = conn.Close()
	}()

	var gotNetwork string
	var gotAddr string
	app := New(Options{
		Dial: func(ctx context.Context, network, addr string) (net.Conn, error) {
			gotNetwork = network
			gotAddr = addr
			var dialer net.Dialer
			return dialer.DialContext(ctx, "tcp", ln.Addr().String())
		},
		Logger: log.New(io.Discard, "", 0),
	})
	u, err := url.Parse("tcp://example.com:8118")
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}

	conn, err := app.connectTCP(u)
	if err != nil {
		t.Fatalf("connect tcp: %v", err)
	}
	_ = conn.Close()

	if gotNetwork != "tcp" {
		t.Fatalf("network = %q, want %q", gotNetwork, "tcp")
	}
	if gotAddr != "example.com:8118" {
		t.Fatalf("addr = %q, want %q", gotAddr, "example.com:8118")
	}

	select {
	case <-accepted:
	case <-time.After(time.Second):
		t.Fatalf("expected listener to be reached through custom dial")
	}
}

func TestConnectWSUsesCustomDial(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := transport.WSUpgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		_ = conn.Close()
	}))
	defer server.Close()

	serverAddr := strings.TrimPrefix(server.URL, "http://")
	var gotNetwork string
	var gotAddr string
	app := New(Options{
		Dial: func(ctx context.Context, network, addr string) (net.Conn, error) {
			gotNetwork = network
			gotAddr = addr
			var dialer net.Dialer
			return dialer.DialContext(ctx, "tcp", serverAddr)
		},
		Logger: log.New(io.Discard, "", 0),
	})
	u, err := url.Parse("ws://gpipe.example:8119")
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}

	conn, err := app.connectWS(u)
	if err != nil {
		t.Fatalf("connect ws: %v", err)
	}
	_ = conn.Close()

	if gotNetwork != "tcp" {
		t.Fatalf("network = %q, want %q", gotNetwork, "tcp")
	}
	if gotAddr != "gpipe.example:8119" {
		t.Fatalf("addr = %q, want %q", gotAddr, "gpipe.example:8119")
	}
}

func TestConnectRejectsUnsupportedSchemeWhenCustomDialIsSet(t *testing.T) {
	app := New(Options{
		Dial: func(ctx context.Context, network, addr string) (net.Conn, error) {
			_ = ctx
			_ = network
			_ = addr
			return nil, nil
		},
		Logger: log.New(io.Discard, "", 0),
	})
	u, err := url.Parse("kcp://127.0.0.1:8118")
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}

	_, err = app.connect(u)
	if err == nil || !strings.Contains(err.Error(), "custom dial only supports tcp:// and ws:// transports") {
		t.Fatalf("err = %v, want custom dial restriction", err)
	}
}

func TestValidateStreamDialServerList(t *testing.T) {
	if err := ValidateStreamDialServerList("tcp://127.0.0.1:8118,ws://127.0.0.1:8119"); err != nil {
		t.Fatalf("validate stream dial server list: %v", err)
	}

	err := ValidateStreamDialServerList("tcp://127.0.0.1:8118,quic://127.0.0.1:8119")
	if err == nil || !strings.Contains(err.Error(), "quic:// is not supported") {
		t.Fatalf("err = %v, want unsupported scheme", err)
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
