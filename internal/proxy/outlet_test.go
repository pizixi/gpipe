package proxy

import (
	"context"
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

func TestOutletStopCancelsPendingConnect(t *testing.T) {
	logger := log.New(io.Discard, "", 0)
	started := make(chan struct{})
	canceled := make(chan struct{})
	outlet := NewOutlet(logger, func(ProxyMessage) {}, "test")
	outlet.dialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		close(started)
		<-ctx.Done()
		close(canceled)
		return nil, ctx.Err()
	}
	outlet.Input(I2OConnect{TunnelID: 1, ID: 99, IsTCP: true, Addr: "127.0.0.1:9", EncryptionKey: EncodeKeyToBase64([]byte("None"))})

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("pending connect did not start")
	}
	if err := outlet.Stop(); err != nil {
		t.Fatalf("stop outlet: %v", err)
	}
	select {
	case <-canceled:
	case <-time.After(time.Second):
		t.Fatal("pending connect was not canceled")
	}
	outlet.mu.RLock()
	defer outlet.mu.RUnlock()
	if len(outlet.sessions) != 0 || len(outlet.connecting) != 0 {
		t.Fatalf("outlet retained sessions=%d connecting=%d after stop", len(outlet.sessions), len(outlet.connecting))
	}
}

func TestOutletDisconnectCancelsPendingConnect(t *testing.T) {
	logger := log.New(io.Discard, "", 0)
	started := make(chan struct{})
	canceled := make(chan struct{})
	outputs := make(chan ProxyMessage, 1)
	outlet := NewOutlet(logger, func(message ProxyMessage) { outputs <- message }, "test")
	outlet.dialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		close(started)
		<-ctx.Done()
		close(canceled)
		return nil, ctx.Err()
	}
	msg := I2OConnect{TunnelID: 1, ID: 101, IsTCP: true, Addr: "127.0.0.1:9", EncryptionKey: EncodeKeyToBase64([]byte("None"))}
	outlet.Input(msg)

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("pending connect did not start")
	}
	outlet.Input(I2ODisconnect{TunnelID: msg.TunnelID, ID: msg.ID})
	select {
	case <-canceled:
	case <-time.After(time.Second):
		t.Fatal("disconnect did not cancel pending connect")
	}
	if err := outlet.Stop(); err != nil {
		t.Fatal(err)
	}
	select {
	case output := <-outputs:
		t.Fatalf("canceled connect emitted stale output: %#v", output)
	default:
	}
}

func TestOutletCanceledDialCannotInstallLateConnection(t *testing.T) {
	logger := log.New(io.Discard, "", 0)
	started := make(chan struct{})
	releaseDial := make(chan struct{})
	outputs := make(chan ProxyMessage, 1)
	outletConn, targetConn := net.Pipe()
	defer targetConn.Close()
	outlet := NewOutlet(logger, func(message ProxyMessage) { outputs <- message }, "test")
	outlet.dialContext = func(context.Context, string, string) (net.Conn, error) {
		close(started)
		<-releaseDial
		return outletConn, nil
	}
	msg := I2OConnect{TunnelID: 1, ID: 103, IsTCP: true, Addr: "127.0.0.1:9", EncryptionKey: EncodeKeyToBase64([]byte("None"))}
	outlet.Input(msg)

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("pending connect did not start")
	}
	outlet.Input(I2ODisconnect{TunnelID: msg.TunnelID, ID: msg.ID})
	close(releaseDial)
	if err := outlet.Stop(); err != nil {
		t.Fatal(err)
	}
	if _, err := targetConn.Read(make([]byte, 1)); err == nil {
		t.Fatal("late target connection remained open after canceled dial")
	}
	select {
	case output := <-outputs:
		t.Fatalf("late canceled connection emitted stale output: %#v", output)
	default:
	}
}

func TestOutletOldConnectAttemptCannotCancelReusedSessionID(t *testing.T) {
	outlet := NewOutlet(log.New(io.Discard, "", 0), func(ProxyMessage) {}, "test")
	_, cancelOld := context.WithCancel(context.Background())
	newContext, cancelNew := context.WithCancel(context.Background())
	defer cancelNew()
	oldAttempt := &outletConnectAttempt{cancel: cancelOld}
	newAttempt := &outletConnectAttempt{cancel: cancelNew}
	outlet.connecting[102] = newAttempt

	outlet.finishConnectAttempt(102, oldAttempt)

	outlet.mu.RLock()
	got := outlet.connecting[102]
	outlet.mu.RUnlock()
	if got != newAttempt {
		t.Fatal("finishing old attempt removed the new attempt for a reused session ID")
	}
	select {
	case <-newContext.Done():
		t.Fatal("finishing old attempt canceled the new attempt")
	default:
	}
	if err := outlet.Stop(); err != nil {
		t.Fatal(err)
	}
}

func TestOutletRejectsConcurrentDuplicateConnect(t *testing.T) {
	logger := log.New(io.Discard, "", 0)
	started := make(chan struct{})
	release := make(chan struct{})
	outputs := make(chan ProxyMessage, 2)
	outlet := NewOutlet(logger, func(message ProxyMessage) { outputs <- message }, "test")
	outlet.dialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		close(started)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-release:
			return nil, errors.New("released test dial")
		}
	}
	msg := I2OConnect{TunnelID: 1, ID: 100, IsTCP: true, Addr: "127.0.0.1:9", EncryptionKey: EncodeKeyToBase64([]byte("None"))}
	outlet.Input(msg)
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("first connect did not start")
	}
	outlet.Input(msg)
	select {
	case output := <-outputs:
		reply, ok := output.(O2IConnect)
		if !ok || reply.Success || reply.ErrorInfo != "repeated connection" {
			t.Fatalf("duplicate reply = %#v", output)
		}
	case <-time.After(time.Second):
		t.Fatal("duplicate connect was not rejected")
	}
	close(release)
	if err := outlet.Stop(); err != nil {
		t.Fatal(err)
	}
}
