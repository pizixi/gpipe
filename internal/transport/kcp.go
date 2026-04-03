package transport

import (
	"log"

	kcp "github.com/xtaci/kcp-go/v5"
)

const (
	kcpMTU          = 1400
	kcpWindowSize   = 1024
	kcpNoDelay      = 1
	kcpInterval     = 10
	kcpResend       = 2
	kcpNoCongestion = 1
	kcpSocketBuffer = 4 * 1024 * 1024
)

// TuneKCPListener 对齐 Rust 侧的常用监听参数，并尽量扩大 UDP socket 缓冲区。
func TuneKCPListener(logger *log.Logger, listener *kcp.Listener) {
	if listener == nil {
		return
	}
	if err := listener.SetReadBuffer(kcpSocketBuffer); err != nil && logger != nil {
		logger.Printf("kcp listener set read buffer failed: %v", err)
	}
	if err := listener.SetWriteBuffer(kcpSocketBuffer); err != nil && logger != nil {
		logger.Printf("kcp listener set write buffer failed: %v", err)
	}
}

// TuneKCPSession 将 kcp-go 参数拉近 Rust 基线，避免默认参数导致行为漂移。
func TuneKCPSession(logger *log.Logger, conn *kcp.UDPSession) {
	if conn == nil {
		return
	}
	conn.SetStreamMode(true)
	conn.SetWindowSize(kcpWindowSize, kcpWindowSize)
	conn.SetNoDelay(kcpNoDelay, kcpInterval, kcpResend, kcpNoCongestion)
	conn.SetACKNoDelay(false)
	if !conn.SetMtu(kcpMTU) && logger != nil {
		logger.Printf("kcp session set mtu failed: mtu=%d", kcpMTU)
	}
}

// TuneKCPDialSession 在公共会话参数之外，额外给主动拨号侧放大 UDP socket 缓冲区。
func TuneKCPDialSession(logger *log.Logger, conn *kcp.UDPSession) {
	TuneKCPSession(logger, conn)
	if conn == nil {
		return
	}
	if err := conn.SetReadBuffer(kcpSocketBuffer); err != nil && logger != nil {
		logger.Printf("kcp session set read buffer failed: %v", err)
	}
	if err := conn.SetWriteBuffer(kcpSocketBuffer); err != nil && logger != nil {
		logger.Printf("kcp session set write buffer failed: %v", err)
	}
}
