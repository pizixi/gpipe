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
	"testing"
	"time"

	"github.com/pizixi/gpipe/internal/db"
	"github.com/pizixi/gpipe/internal/manager"
	"github.com/pizixi/gpipe/internal/pb"
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
