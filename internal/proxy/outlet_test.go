package proxy

import (
	"errors"
	"io"
	"log"
	"net"
	"sync/atomic"
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

type failingPeerWriter struct {
	err        error
	closeCount atomic.Int32
}

func (w *failingPeerWriter) Write([]byte, SendResultHook) error {
	return w.err
}

func (w *failingPeerWriter) WriteTo([]byte, net.Addr) error {
	return w.err
}

func (w *failingPeerWriter) Close() error {
	w.closeCount.Add(1)
	return nil
}

func TestOutletTerminatesSessionOnWriteFailure(t *testing.T) {
	logger := log.New(io.Discard, "", 0)
	messages := make(chan ProxyMessage, 4)
	outlet := NewOutlet(logger, func(msg ProxyMessage) {
		messages <- msg
	}, "test")

	writer := &failingPeerWriter{err: errors.New("write failed")}
	common := NewSessionCommonInfo(false, ParseEncryptionMethod("None"), nil)
	outlet.putSession(2, &outletSession{
		id:     2,
		writer: writer,
		common: common,
		close:  func() { _ = writer.Close() },
		inputQ: newProxyMessageQueue(),
	})

	payload, err := common.EncodeDataAndLimit([]byte("hello"))
	if err != nil {
		t.Fatalf("encode payload: %v", err)
	}
	outlet.Input(I2OSendData{TunnelID: 1, ID: 2, Data: payload})

	select {
	case msg := <-messages:
		disconnect, ok := msg.(O2IDisconnect)
		if !ok {
			t.Fatalf("expected O2IDisconnect, got %T", msg)
		}
		if disconnect.ID != 2 || disconnect.TunnelID != 1 {
			t.Fatalf("unexpected disconnect message: %+v", disconnect)
		}
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for disconnect")
	}

	if _, ok := outlet.session(2); ok {
		t.Fatalf("expected session to be removed after write failure")
	}
	if writer.closeCount.Load() == 0 {
		t.Fatalf("expected writer to be closed after write failure")
	}
}

func TestOutletTerminatesSessionWhenInputQueueOverflows(t *testing.T) {
	logger := log.New(io.Discard, "", 0)
	messages := make(chan ProxyMessage, 4)
	outlet := NewOutlet(logger, func(msg ProxyMessage) {
		messages <- msg
	}, "test")

	writer := &failingPeerWriter{}
	queue := newProxyMessageQueueWithCapacity(1)
	if !queue.Push(I2ODisconnect{TunnelID: 1, ID: 3}) {
		t.Fatalf("expected queue prefill to succeed")
	}
	session := &outletSession{
		id:     3,
		writer: writer,
		common: NewSessionCommonInfo(false, ParseEncryptionMethod("None"), nil),
		close:  func() { _ = writer.Close() },
		inputQ: queue,
	}

	outlet.mu.Lock()
	outlet.sessions[3] = session
	outlet.mu.Unlock()

	outlet.Input(I2OSendData{TunnelID: 1, ID: 3, Data: []byte("payload")})

	select {
	case msg := <-messages:
		disconnect, ok := msg.(O2IDisconnect)
		if !ok {
			t.Fatalf("expected O2IDisconnect, got %T", msg)
		}
		if disconnect.ID != 3 || disconnect.TunnelID != 1 {
			t.Fatalf("unexpected disconnect message: %+v", disconnect)
		}
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for queue overflow disconnect")
	}

	if _, ok := outlet.session(3); ok {
		t.Fatalf("expected session to be removed after queue overflow")
	}
	if writer.closeCount.Load() == 0 {
		t.Fatalf("expected writer to be closed after queue overflow")
	}
}
