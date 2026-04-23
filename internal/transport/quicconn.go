package transport

import (
	"net"
	"sync"
	"time"

	quic "github.com/quic-go/quic-go"
)

// QUICConn 用单条双向流模拟 net.Conn。
type QUICConn struct {
	stream    *quic.Stream
	local     net.Addr
	remote    net.Addr
	close     func() error
	closeOnce sync.Once
}

func NewQUICConn(stream *quic.Stream, local, remote net.Addr, closeFn func() error) *QUICConn {
	return &QUICConn{
		stream: stream,
		local:  local,
		remote: remote,
		close:  closeFn,
	}
}

func (c *QUICConn) Read(p []byte) (int, error)  { return c.stream.Read(p) }
func (c *QUICConn) Write(p []byte) (int, error) { return c.stream.Write(p) }
func (c *QUICConn) LocalAddr() net.Addr         { return c.local }
func (c *QUICConn) RemoteAddr() net.Addr        { return c.remote }
func (c *QUICConn) SetDeadline(t time.Time) error {
	if err := c.stream.SetReadDeadline(t); err != nil {
		return err
	}
	return c.stream.SetWriteDeadline(t)
}
func (c *QUICConn) SetReadDeadline(t time.Time) error  { return c.stream.SetReadDeadline(t) }
func (c *QUICConn) SetWriteDeadline(t time.Time) error { return c.stream.SetWriteDeadline(t) }
func (c *QUICConn) Close() error {
	var err error
	c.closeOnce.Do(func() {
		err = c.stream.Close()
		if c.close != nil {
			_ = c.close()
		}
	})
	return err
}
