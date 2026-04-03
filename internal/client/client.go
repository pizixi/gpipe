package client

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pizixi/gpipe/internal/codec"
	"github.com/pizixi/gpipe/internal/pb"
	"github.com/pizixi/gpipe/internal/proxy"
	"github.com/pizixi/gpipe/internal/transport"

	"github.com/gorilla/websocket"
	quic "github.com/quic-go/quic-go"
	kcp "github.com/xtaci/kcp-go/v5"
)

var tlsHandshakeTimeout = 15 * time.Second
var connectTimeout = 10 * time.Second

const readCacheCompactThreshold = 256 * 1024

type Options struct {
	Server        string
	Key           string
	EnableTLS     bool
	TLSServerName string
	Insecure      bool
	CACert        string
	Logger        *log.Logger
	// Dial 可选的自定义流式拨号函数。
	// - nil：直连（不使用任何代理）
	// - 非 nil：通过该函数拨号（例如 Shadowsocks、SOCKS5）
	// 仅对基于 TCP 的传输生效（tcp://、ws://）；quic://、kcp:// 仍使用 UDP 原生拨号。
	Dial DialFunc
}

type App struct {
	opts Options
}

type clientSession struct {
	opts       Options
	conn       net.Conn
	writeMu    sync.Mutex
	playerID   uint32
	lastActive atomic.Int64
	tunnels    map[uint32]*pb.Tunnel
	proxyMgr   *proxy.Manager
	closeOnce  sync.Once
}

func New(opts Options) *App {
	return &App{
		opts: opts,
	}
}

func (a *App) Run() error {
	return a.RunContext(context.Background())
}

func (a *App) RunContext(ctx context.Context) error {
	uris := splitURIs(a.opts.Server)
	if len(uris) == 0 {
		return fmt.Errorf("no valid uri found")
	}
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		for _, raw := range uris {
			select {
			case <-ctx.Done():
				return nil
			default:
			}
			u, err := url.Parse(raw)
			if err != nil {
				a.opts.Logger.Printf("skip invalid server uri %q: %v", raw, err)
				continue
			}
			if err := a.runOne(ctx, u); err != nil {
				a.opts.Logger.Printf("client run error: %v", err)
				select {
				case <-ctx.Done():
					return nil
				case <-time.After(15 * time.Second):
				}
			} else {
				select {
				case <-ctx.Done():
					return nil
				case <-time.After(1 * time.Second):
				}
			}
		}
	}
}

func (a *App) runOne(ctx context.Context, u *url.URL) error {
	conn, err := a.connect(u)
	if err != nil {
		return err
	}
	session := newClientSession(a.opts, conn)
	defer session.close()

	if err := session.send(-1, &pb.LoginReq{
		Version:  "0.0.0",
		Password: session.opts.Key,
	}); err != nil {
		return err
	}

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	errCh := make(chan error, 2)
	go func() { errCh <- session.readLoop(runCtx) }()
	go func() { errCh <- session.pingLoop(runCtx) }()
	select {
	case <-ctx.Done():
		return nil
	case err := <-errCh:
		cancel()
		return err
	}
}

func (a *App) connect(u *url.URL) (net.Conn, error) {
	if a.opts.Dial != nil {
		if err := validateStreamDialScheme(u.Scheme); err != nil {
			return nil, err
		}
	}
	switch u.Scheme {
	case "tcp":
		return a.connectTCP(u)
	case "ws":
		return a.connectWS(u)
	case "quic":
		return a.connectQUIC(u)
	case "kcp":
		return a.connectKCP(u)
	default:
		return nil, fmt.Errorf("unsupported scheme: %s", u.String())
	}
}

func (a *App) connectTCP(u *url.URL) (net.Conn, error) {
	addr := u.Host
	ctx, cancel := context.WithTimeout(context.Background(), connectTimeout)
	defer cancel()
	conn, err := a.streamDialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}
	if !a.opts.EnableTLS {
		return conn, nil
	}

	cfg, err := a.tlsConfig(u)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	return handshakeClientTLSConn(conn, cfg)
}

func (a *App) connectWS(u *url.URL) (net.Conn, error) {
	scheme := "ws"
	if a.opts.EnableTLS {
		scheme = "wss"
	}
	target := &url.URL{Scheme: scheme, Host: u.Host, Path: "/"}
	dialer := websocket.Dialer{
		HandshakeTimeout: tlsHandshakeTimeout,
		NetDialContext:   a.websocketDialContext,
	}
	if a.opts.EnableTLS {
		tlsCfg, err := a.tlsConfig(u)
		if err != nil {
			return nil, err
		}
		dialer.TLSClientConfig = tlsCfg
	}
	ctx, cancel := context.WithTimeout(context.Background(), connectTimeout+tlsHandshakeTimeout)
	defer cancel()
	conn, _, err := dialer.DialContext(ctx, target.String(), nil)
	if err != nil {
		return nil, err
	}
	return transport.NewWSConn(conn), nil
}

func (a *App) connectQUIC(u *url.URL) (net.Conn, error) {
	if !a.opts.EnableTLS {
		return nil, fmt.Errorf("QUIC protocol requires Transport Layer Security (TLS) to be enabled for secure communication.")
	}
	tlsCfg, err := a.tlsConfig(u)
	if err != nil {
		return nil, err
	}
	tlsCfg.NextProtos = []string{"npipe", "h3"}
	ctx, cancel := context.WithTimeout(context.Background(), tlsHandshakeTimeout)
	defer cancel()
	conn, err := quic.DialAddr(ctx, u.Host, tlsCfg, nil)
	if err != nil {
		return nil, err
	}
	stream, err := conn.OpenStreamSync(ctx)
	if err != nil {
		_ = conn.CloseWithError(0, "")
		return nil, err
	}
	return transport.NewQUICConn(stream, conn.LocalAddr(), conn.RemoteAddr(), func() error {
		return conn.CloseWithError(0, "")
	}), nil
}

func (a *App) connectKCP(u *url.URL) (net.Conn, error) {
	conn, err := kcp.DialWithOptions(u.Host, nil, 10, 3)
	if err != nil {
		return nil, err
	}
	transport.TuneKCPDialSession(a.opts.Logger, conn)
	if !a.opts.EnableTLS {
		return conn, nil
	}
	tlsCfg, err := a.tlsConfig(u)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	return handshakeClientTLSConn(conn, tlsCfg)
}

func (a *App) tlsConfig(u *url.URL) (*tls.Config, error) {
	cfg := &tls.Config{
		// TLS is used here only to encrypt the transport, so certificate
		// verification is intentionally skipped by default.
		InsecureSkipVerify: true, //nolint:gosec
		MinVersion:         tls.VersionTLS12,
	}
	if a.opts.TLSServerName != "" {
		cfg.ServerName = a.opts.TLSServerName
	} else {
		cfg.ServerName = u.Hostname()
	}
	return cfg, nil
}

func (a *App) streamDialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	if a.opts.Dial != nil {
		return a.opts.Dial(ctx, network, addr)
	}
	return directDial(ctx, network, addr)
}

func (a *App) websocketDialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	if a.opts.Dial != nil {
		return a.opts.Dial(ctx, network, addr)
	}
	return (&net.Dialer{
		Timeout:   connectTimeout,
		KeepAlive: 30 * time.Second,
	}).DialContext(ctx, network, addr)
}

func handshakeClientTLSConn(conn net.Conn, cfg *tls.Config) (net.Conn, error) {
	tlsConn := tls.Client(conn, cfg)
	ctx, cancel := context.WithTimeout(context.Background(), tlsHandshakeTimeout)
	defer cancel()
	if err := tlsConn.HandshakeContext(ctx); err != nil {
		_ = tlsConn.Close()
		return nil, err
	}
	return tlsConn, nil
}

func newClientSession(opts Options, conn net.Conn) *clientSession {
	session := &clientSession{
		opts:    opts,
		conn:    conn,
		tunnels: map[uint32]*pb.Tunnel{},
	}
	session.touch()
	return session
}

func (s *clientSession) close() {
	s.closeOnce.Do(func() {
		if s.proxyMgr != nil {
			s.proxyMgr.Close()
			s.proxyMgr = nil
		}
		if s.conn != nil {
			_ = s.conn.Close()
			s.conn = nil
		}
	})
}

func (s *clientSession) touch() {
	s.lastActive.Store(time.Now().UnixNano())
}

func (s *clientSession) lastSeen() time.Time {
	return time.Unix(0, s.lastActive.Load())
}

func (s *clientSession) readLoop(ctx context.Context) error {
	buf := make([]byte, 4096)
	cache := make([]byte, 0, 64*1024)
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		n, err := s.conn.Read(buf)
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		s.touch()
		cache = append(cache, buf[:n]...)
		for {
			frame, rest, err := codec.TryExtractFrame(cache, 5*1024*1024)
			if err != nil {
				return err
			}
			if frame == nil {
				cache = compactReadCache(rest)
				break
			}
			cache = compactReadCache(rest)
			if err := s.handleFrame(frame); err != nil {
				return err
			}
		}
	}
}

func (s *clientSession) handleFrame(frame []byte) error {
	serial, message, err := codec.Decode(frame)
	if err != nil {
		return err
	}
	if s.playerID == 0 {
		if serial <= 0 {
			return fmt.Errorf("login failed")
		}
		switch msg := message.(type) {
		case *pb.LoginAck:
			s.playerID = msg.PlayerID
			s.tunnels = map[uint32]*pb.Tunnel{}
			for _, tunnel := range msg.TunnelList {
				s.tunnels[tunnel.ID] = tunnel
			}
			if s.proxyMgr != nil {
				s.proxyMgr.Close()
			}
			s.proxyMgr = proxy.NewManager(s.opts.Logger, s.playerID, func(playerID uint32, message any) error {
				return s.send(0, message)
			})
			s.proxyMgr.SyncTunnels(msg.TunnelList)
			s.opts.Logger.Printf("login successful, player id: %d", s.playerID)
			return nil
		case *pb.Error:
			return fmt.Errorf("login failed: %s, code: %d", msg.Message, msg.Number)
		default:
			return fmt.Errorf("login failed, received unknown message")
		}
	}

	if serial == 0 {
		switch msg := message.(type) {
		case *pb.ModifyTunnelNtf:
			if msg.Tunnel == nil {
				return nil
			}
			if msg.IsDelete {
				delete(s.tunnels, msg.Tunnel.ID)
			} else {
				s.tunnels[msg.Tunnel.ID] = msg.Tunnel
			}
			if s.proxyMgr != nil {
				s.proxyMgr.UpdateTunnel(msg)
			}
			s.opts.Logger.Printf("tunnel update: id=%d delete=%v", msg.Tunnel.ID, msg.IsDelete)
		default:
			if s.proxyMgr != nil {
				s.proxyMgr.HandlePB(message)
			}
		}
	}
	return nil
}

func (s *clientSession) pingLoop(ctx context.Context) error {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
		if time.Since(s.lastSeen()) < 5*time.Second {
			continue
		}
		if time.Since(s.lastSeen()) > 15*time.Second {
			return fmt.Errorf("ping timeout")
		}
		if err := s.send(-2, &pb.Ping{Ticks: time.Now().UnixMilli()}); err != nil {
			return err
		}
	}
}

func (s *clientSession) send(serial int32, message any) error {
	packet, err := codec.Encode(serial, message)
	if err != nil {
		return err
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	if s.conn == nil {
		return net.ErrClosed
	}
	return writeAll(s.conn, packet)
}

func splitURIs(value string) []string {
	raw := strings.Split(value, ",")
	var out []string
	for _, item := range raw {
		item = strings.TrimSpace(item)
		if item != "" {
			out = append(out, item)
		}
	}
	return out
}

func writeAll(conn net.Conn, data []byte) error {
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

func compactReadCache(buf []byte) []byte {
	if len(buf) == 0 {
		return nil
	}
	if cap(buf) <= readCacheCompactThreshold || len(buf)*4 >= cap(buf) {
		return buf
	}
	compacted := make([]byte, len(buf))
	copy(compacted, buf)
	return compacted
}
