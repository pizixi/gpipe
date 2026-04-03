package client

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/shadowsocks/go-shadowsocks2/core"
)

// SSDialConfig 是 Shadowsocks 出站拨号配置。
type SSDialConfig struct {
	ServerAddr string
	Method     string
	Password   string
}

// SSDialer 通过 Shadowsocks 服务端建立到目标地址的连接。
type SSDialer struct {
	serverAddr string
	cipher     core.Cipher
}

func NewSSDialer(cfg SSDialConfig) (*SSDialer, error) {
	cipher, err := core.PickCipher(cfg.Method, nil, cfg.Password)
	if err != nil {
		return nil, fmt.Errorf("init shadowsocks cipher: %w", err)
	}
	return &SSDialer{
		serverAddr: cfg.ServerAddr,
		cipher:     cipher,
	}, nil
}

func (d *SSDialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	switch strings.ToLower(strings.TrimSpace(network)) {
	case "tcp", "tcp4", "tcp6":
	default:
		return nil, fmt.Errorf("shadowsocks dial only supports tcp, got %q", network)
	}

	var dialer net.Dialer
	rawConn, err := dialer.DialContext(ctx, "tcp", d.serverAddr)
	if err != nil {
		return nil, fmt.Errorf("connect shadowsocks server %s: %w", d.serverAddr, err)
	}

	ssConn := d.cipher.StreamConn(rawConn)
	header, err := marshalSSAddr(addr)
	if err != nil {
		_ = ssConn.Close()
		return nil, err
	}
	if _, err := ssConn.Write(header); err != nil {
		_ = ssConn.Close()
		return nil, fmt.Errorf("write shadowsocks target addr: %w", err)
	}

	return ssConn, nil
}

func marshalSSAddr(addr string) ([]byte, error) {
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, fmt.Errorf("parse addr %q: %w", addr, err)
	}

	port, err := strconv.ParseUint(portStr, 10, 16)
	if err != nil {
		return nil, fmt.Errorf("parse port %q: %w", portStr, err)
	}

	portBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(portBytes, uint16(port))

	if ip := net.ParseIP(host); ip != nil {
		if ip4 := ip.To4(); ip4 != nil {
			return append(append([]byte{0x01}, ip4...), portBytes...), nil
		}
		return append(append([]byte{0x04}, ip.To16()...), portBytes...), nil
	}

	if len(host) > 255 {
		return nil, fmt.Errorf("domain too long: %d", len(host))
	}
	buf := make([]byte, 0, 1+1+len(host)+2)
	buf = append(buf, 0x03, byte(len(host)))
	buf = append(buf, host...)
	buf = append(buf, portBytes...)
	return buf, nil
}
