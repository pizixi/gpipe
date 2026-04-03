package transport

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
)

func TestWSConnReadStreamsAcrossMessages(t *testing.T) {
	done := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := WSUpgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		if err := conn.WriteMessage(websocket.BinaryMessage, []byte("abc")); err != nil {
			return
		}
		if err := conn.WriteMessage(websocket.TextMessage, []byte("defg")); err != nil {
			return
		}

		<-done
	}))
	defer server.Close()

	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	wsConn := NewWSConn(conn)
	defer wsConn.Close()

	buf := make([]byte, 2)
	got := make([]byte, 0, 7)
	for len(got) < 7 {
		n, err := wsConn.Read(buf)
		if err != nil {
			t.Fatalf("read websocket conn: %v", err)
		}
		got = append(got, buf[:n]...)
	}

	close(done)

	if string(got) != "abcdefg" {
		t.Fatalf("read payload = %q, want %q", string(got), "abcdefg")
	}
}

func TestWSConnWriteSendsBinaryMessage(t *testing.T) {
	type readResult struct {
		messageType int
		data        []byte
		err         error
	}

	resultCh := make(chan readResult, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := WSUpgrader.Upgrade(w, r, nil)
		if err != nil {
			resultCh <- readResult{err: err}
			return
		}
		defer conn.Close()

		messageType, reader, err := conn.NextReader()
		if err != nil {
			resultCh <- readResult{err: err}
			return
		}
		data, err := io.ReadAll(reader)
		resultCh <- readResult{messageType: messageType, data: data, err: err}
	}))
	defer server.Close()

	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	wsConn := NewWSConn(conn)
	defer wsConn.Close()

	payload := []byte("hello websocket")
	n, err := wsConn.Write(payload)
	if err != nil {
		t.Fatalf("write websocket conn: %v", err)
	}
	if n != len(payload) {
		t.Fatalf("write len = %d, want %d", n, len(payload))
	}

	resultValue := <-resultCh
	if resultValue.err != nil {
		t.Fatalf("server read result error: %v", resultValue.err)
	}
	if resultValue.messageType != websocket.BinaryMessage {
		t.Fatalf("message type = %d, want %d", resultValue.messageType, websocket.BinaryMessage)
	}
	if string(resultValue.data) != string(payload) {
		t.Fatalf("payload = %q, want %q", string(resultValue.data), string(payload))
	}
}
