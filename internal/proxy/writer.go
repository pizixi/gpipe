package proxy

import (
	"errors"
	"io"
	"net"
	"strings"
	"sync"
	"time"
)

// TCPWriter 抽象对 TCP 本地连接的写入。
type TCPWriter struct {
	conn net.Conn
	mu   sync.Mutex
}

func NewTCPWriter(conn net.Conn) *TCPWriter {
	return &TCPWriter{conn: conn}
}

func (w *TCPWriter) Write(data []byte, hook SendResultHook) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := writeAllStream(w.conn, data); err != nil {
		return err
	}
	if hook != nil {
		hook()
	}
	return nil
}

func (w *TCPWriter) WriteTo(data []byte, _ net.Addr) error {
	return w.Write(data, nil)
}

func (w *TCPWriter) Close() error {
	return w.conn.Close()
}

// UDPWriter 用于对 UDP socket 回写。
type UDPWriter struct {
	socket *net.UDPConn
	peer   *net.UDPAddr
	mu     sync.Mutex
}

func NewUDPWriter(socket *net.UDPConn, peer *net.UDPAddr) *UDPWriter {
	return &UDPWriter{socket: socket, peer: peer}
}

func (w *UDPWriter) Write(data []byte, hook SendResultHook) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	var err error
	if w.peer != nil {
		_, err = w.socket.WriteToUDP(data, w.peer)
	} else {
		_, err = w.socket.Write(data)
	}
	if err != nil {
		return err
	}
	if hook != nil {
		hook()
	}
	return nil
}

func (w *UDPWriter) WriteTo(data []byte, addr net.Addr) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	target, _ := addr.(*net.UDPAddr)
	if target == nil {
		target = w.peer
	}
	if target == nil {
		_, err := w.socket.Write(data)
		return err
	}
	_, err := w.socket.WriteToUDP(data, target)
	return err
}

func (w *UDPWriter) Close() error {
	// UDP 会话不关闭共享 socket，只做空操作。
	return nil
}

// CloseLater 与 Rust 中 CloseDelayed 类似。
func CloseLater(closer interface{ Close() error }, d time.Duration) {
	go func() {
		time.Sleep(d)
		_ = closer.Close()
	}()
}

func writeAllStream(conn net.Conn, data []byte) error {
	for len(data) > 0 {
		n, err := conn.Write(data)
		if n > 0 {
			data = data[n:]
		}
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
	}
	return nil
}

func isExpectedNetCloseError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, net.ErrClosed) {
		return true
	}
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "use of closed network connection") ||
		strings.Contains(text, "forcibly closed by the remote host") ||
		strings.Contains(text, "connection reset by peer") ||
		strings.Contains(text, "broken pipe")
}

func configureTCPConn(conn *net.TCPConn) error {
	if conn == nil {
		return nil
	}
	if err := conn.SetNoDelay(true); err != nil {
		return err
	}
	if err := conn.SetKeepAlive(true); err != nil {
		return err
	}
	return conn.SetKeepAlivePeriod(30 * time.Second)
}
