package client

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strings"
)

// DialFunc 通用流式拨号函数类型。
// 适用于基于 TCP 的传输（tcp://、ws://）。
type DialFunc func(ctx context.Context, network, addr string) (net.Conn, error)

// directDial 默认直连，不走任何代理
func directDial(ctx context.Context, network, addr string) (net.Conn, error) {
	var d net.Dialer
	return d.DialContext(ctx, network, addr)
}

// ValidateStreamDialServerList 校验自定义流式拨号可用的服务端地址列表。
// 当前只有 tcp:// 和 ws:// 可以承载 DialFunc。
func ValidateStreamDialServerList(server string) error {
	for _, raw := range splitURIs(server) {
		u, err := url.Parse(raw)
		if err != nil {
			return fmt.Errorf("invalid server uri %q: %w", raw, err)
		}
		if err := validateStreamDialScheme(u.Scheme); err != nil {
			return err
		}
	}
	return nil
}

func validateStreamDialScheme(scheme string) error {
	switch strings.ToLower(strings.TrimSpace(scheme)) {
	case "tcp", "ws":
		return nil
	default:
		return fmt.Errorf(
			"custom dial only supports tcp:// and ws:// transports; %s:// is not supported",
			scheme,
		)
	}
}
