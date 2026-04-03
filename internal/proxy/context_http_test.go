package proxy

import (
	"bytes"
	"net"
	"strings"
	"sync"
	"testing"
)

type testPeerWriter struct {
	mu     sync.Mutex
	writes [][]byte
	closed int
}

func (w *testPeerWriter) Write(data []byte, hook SendResultHook) error {
	w.mu.Lock()
	w.writes = append(w.writes, append([]byte(nil), data...))
	w.mu.Unlock()
	if hook != nil {
		hook()
	}
	return nil
}

func (w *testPeerWriter) WriteTo(data []byte, _ net.Addr) error {
	return w.Write(data, nil)
}

func (w *testPeerWriter) Close() error {
	w.mu.Lock()
	w.closed++
	w.mu.Unlock()
	return nil
}

func (w *testPeerWriter) lastWrite() []byte {
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.writes) == 0 {
		return nil
	}
	return append([]byte(nil), w.writes[len(w.writes)-1]...)
}

func TestHTTPContextForwardsHTTPRequestAfterConnect(t *testing.T) {
	var outputs []ProxyMessage
	ctxData := NewContextData(
		7,
		TunnelModeHTTP,
		"",
		func(msg ProxyMessage) {
			outputs = append(outputs, msg)
		},
		NewSessionCommonInfo(false, ParseEncryptionMethod("None"), nil),
		InletAuthData{},
	)
	ctxData.SetSessionID(9)

	writer := &testPeerWriter{}
	ctx := NewHTTPContext()
	if err := ctx.OnStart(ctxData, &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 3456}, writer); err != nil {
		t.Fatalf("OnStart failed: %v", err)
	}

	request := "GET http://example.com/test HTTP/1.1\r\nHost: example.com\r\nProxy-Connection: keep-alive\r\nProxy-Authorization: Basic abc\r\nVia: old\r\n\r\n"
	if err := ctx.OnPeerData(ctxData, []byte(request)); err != nil {
		t.Fatalf("OnPeerData failed: %v", err)
	}
	if len(outputs) != 1 {
		t.Fatalf("expected 1 output after request parse, got %d", len(outputs))
	}
	connect, ok := outputs[0].(I2OConnect)
	if !ok {
		t.Fatalf("expected I2OConnect, got %T", outputs[0])
	}
	if connect.Addr != "example.com:80" {
		t.Fatalf("unexpected connect addr: %s", connect.Addr)
	}

	if err := ctx.OnProxyMessage(O2IConnect{TunnelID: 7, ID: 9, Success: true}); err != nil {
		t.Fatalf("OnProxyMessage connect ack failed: %v", err)
	}
	if len(outputs) != 2 {
		t.Fatalf("expected 2 outputs after connect ack, got %d", len(outputs))
	}
	send, ok := outputs[1].(I2OSendData)
	if !ok {
		t.Fatalf("expected I2OSendData, got %T", outputs[1])
	}
	decoded, err := ctxData.common.DecodeData(send.Data)
	if err != nil {
		t.Fatalf("DecodeData failed: %v", err)
	}
	text := string(decoded)
	if !strings.HasPrefix(text, "GET /test HTTP/1.1\r\n") {
		t.Fatalf("unexpected rewritten request line: %q", text)
	}
	if strings.Contains(text, "Proxy-Authorization") || strings.Contains(text, "Proxy-Connection") || strings.Contains(text, "Via:") {
		t.Fatalf("proxy-specific headers were not removed: %q", text)
	}
}

func TestHTTPContextConnectWritesEstablishedResponse(t *testing.T) {
	var outputs []ProxyMessage
	ctxData := NewContextData(
		8,
		TunnelModeHTTP,
		"",
		func(msg ProxyMessage) {
			outputs = append(outputs, msg)
		},
		NewSessionCommonInfo(false, ParseEncryptionMethod("None"), nil),
		InletAuthData{},
	)
	ctxData.SetSessionID(10)

	writer := &testPeerWriter{}
	ctx := NewHTTPContext()
	if err := ctx.OnStart(ctxData, &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 4567}, writer); err != nil {
		t.Fatalf("OnStart failed: %v", err)
	}

	request := "CONNECT example.com:443 HTTP/1.1\r\nHost: example.com:443\r\n\r\n"
	if err := ctx.OnPeerData(ctxData, []byte(request)); err != nil {
		t.Fatalf("OnPeerData failed: %v", err)
	}
	if len(outputs) != 1 {
		t.Fatalf("expected connect output, got %d", len(outputs))
	}
	connect, ok := outputs[0].(I2OConnect)
	if !ok {
		t.Fatalf("expected I2OConnect, got %T", outputs[0])
	}
	if connect.Addr != "example.com:443" {
		t.Fatalf("unexpected CONNECT addr: %s", connect.Addr)
	}

	if err := ctx.OnProxyMessage(O2IConnect{TunnelID: 8, ID: 10, Success: true}); err != nil {
		t.Fatalf("OnProxyMessage failed: %v", err)
	}
	response := writer.lastWrite()
	if !bytes.Contains(response, []byte("200 Connection Established")) {
		t.Fatalf("expected CONNECT established response, got %q", string(response))
	}
}
