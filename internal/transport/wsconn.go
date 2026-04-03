package transport

import (
	"errors"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// WSConn 把 websocket 封装成 net.Conn，便于复用统一会话逻辑。
type WSConn struct {
	conn       *websocket.Conn
	reader     io.Reader
	readMu     sync.Mutex
	writeMu    sync.Mutex
	localAddr  net.Addr
	remoteAddr net.Addr
}

func NewWSConn(conn *websocket.Conn) *WSConn {
	return &WSConn{
		conn:       conn,
		localAddr:  conn.LocalAddr(),
		remoteAddr: conn.RemoteAddr(),
	}
}

func (c *WSConn) Read(p []byte) (int, error) {
	c.readMu.Lock()
	defer c.readMu.Unlock()
	for {
		if c.reader == nil {
			mt, reader, err := c.conn.NextReader()
			if err != nil {
				return 0, err
			}
			if mt != websocket.BinaryMessage && mt != websocket.TextMessage {
				continue
			}
			c.reader = reader
		}
		n, err := c.reader.Read(p)
		if errors.Is(err, io.EOF) {
			c.reader = nil
			if n > 0 {
				return n, nil
			}
			continue
		}
		return n, err
	}
}

func (c *WSConn) Write(p []byte) (int, error) {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	writer, err := c.conn.NextWriter(websocket.BinaryMessage)
	if err != nil {
		return 0, err
	}
	n, writeErr := writeAll(writer, p)
	closeErr := writer.Close()
	if writeErr != nil {
		return n, writeErr
	}
	if closeErr != nil {
		return n, closeErr
	}
	return n, nil
}

func (c *WSConn) Close() error {
	return c.conn.Close()
}

func (c *WSConn) LocalAddr() net.Addr  { return c.localAddr }
func (c *WSConn) RemoteAddr() net.Addr { return c.remoteAddr }

func (c *WSConn) SetDeadline(t time.Time) error {
	if err := c.conn.SetReadDeadline(t); err != nil {
		return err
	}
	return c.conn.SetWriteDeadline(t)
}

func (c *WSConn) SetReadDeadline(t time.Time) error {
	return c.conn.SetReadDeadline(t)
}

func (c *WSConn) SetWriteDeadline(t time.Time) error {
	return c.conn.SetWriteDeadline(t)
}

var WSUpgrader = websocket.Upgrader{
	CheckOrigin: func(*http.Request) bool { return true },
}

func writeAll(writer io.Writer, data []byte) (int, error) {
	total := 0
	for len(data) > 0 {
		n, err := writer.Write(data)
		total += n
		if n > 0 {
			data = data[n:]
		}
		if err != nil {
			return total, err
		}
		if n == 0 {
			return total, io.ErrShortWrite
		}
	}
	return total, nil
}
