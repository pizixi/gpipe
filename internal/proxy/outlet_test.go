package proxy

import (
	"io"
	"log"
	"net"
	"testing"
	"time"
)

func TestReadTCPSendsRecvDataForSocks5Connect(t *testing.T) {
	logger := log.New(io.Discard, "", 0)
	messages := make(chan ProxyMessage, 8)
	outlet := &Outlet{
		logger:      logger,
		description: "test",
		output: func(msg ProxyMessage) {
			messages <- msg
		},
		sessions: map[uint32]*outletSession{},
	}

	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	common := NewSessionCommonInfo(false, ParseEncryptionMethod("None"), nil)
	done := make(chan struct{})
	go func() {
		outlet.readTCP(1, 2, clientConn, common, true)
		close(done)
	}()

	payload := []byte("hello over socks5 tcp")
	if _, err := serverConn.Write(payload); err != nil {
		t.Fatalf("write pipe: %v", err)
	}

	select {
	case msg := <-messages:
		recv, ok := msg.(O2IRecvData)
		if !ok {
			t.Fatalf("expected O2IRecvData, got %T", msg)
		}
		decoded, err := common.DecodeData(recv.Data)
		if err != nil {
			t.Fatalf("decode data: %v", err)
		}
		if string(decoded) != string(payload) {
			t.Fatalf("decoded payload mismatch, got %q want %q", string(decoded), string(payload))
		}
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for tcp recv data")
	}

	_ = serverConn.Close()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatalf("readTCP did not exit")
	}
}
