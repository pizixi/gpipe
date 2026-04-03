package proxy

import (
	"fmt"
	"log"
	"net"
	"sync"
	"time"
)

type outletSession struct {
	id     uint32
	writer PeerWriter
	common *SessionCommonInfo
	close  func()
	inputQ *proxyMessageQueue
	once   sync.Once
}

// Outlet 负责连接出口目标，并把远端返回的数据转成代理消息。
type Outlet struct {
	logger      *log.Logger
	description string
	output      OutputFunc

	mu       sync.RWMutex
	sessions map[uint32]*outletSession
}

func NewOutlet(logger *log.Logger, output OutputFunc, description string) *Outlet {
	return &Outlet{
		logger:      logger,
		description: description,
		output:      output,
		sessions:    map[uint32]*outletSession{},
	}
}

func (o *Outlet) Description() string {
	return o.description
}

func (o *Outlet) Stop() error {
	o.mu.Lock()
	defer o.mu.Unlock()
	for _, session := range o.sessions {
		session.common.Close()
		session.stopInput()
		if session.close != nil {
			session.close()
		}
	}
	o.sessions = map[uint32]*outletSession{}
	return nil
}

func (o *Outlet) Input(message ProxyMessage) {
	switch msg := message.(type) {
	case I2OConnect:
		safeGo(o.logger, goroutineName("outlet-connect-", msg.ID), func() {
			o.onConnect(msg)
		})
	case I2OSendData:
		if session, ok := o.session(msg.ID); ok {
			if !session.inputQ.Push(msg) {
				o.logger.Printf("出口会话消息被丢弃: session=%d", msg.ID)
			}
		}
	case I2OSendToData:
		if session, ok := o.session(msg.ID); ok {
			if !session.inputQ.Push(msg) {
				o.logger.Printf("出口会话消息被丢弃: session=%d", msg.ID)
			}
		}
	case I2ODisconnect:
		if session, ok := o.session(msg.ID); ok {
			if !session.inputQ.Push(msg) {
				o.logger.Printf("出口会话消息被丢弃: session=%d", msg.ID)
			}
		}
	case I2ORecvDataResult:
		o.onRecvResult(msg)
	}
}

func (o *Outlet) onConnect(msg I2OConnect) {
	if _, ok := o.session(msg.ID); ok {
		o.output(O2IConnect{TunnelID: msg.TunnelID, ID: msg.ID, Success: false, ErrorInfo: "repeated connection"})
		return
	}

	method := ParseEncryptionMethod(msg.EncryptionMethod)
	key, err := DecodeKeyFromBase64(msg.EncryptionKey)
	if err != nil {
		o.output(O2IConnect{TunnelID: msg.TunnelID, ID: msg.ID, Success: false, ErrorInfo: err.Error()})
		return
	}
	common := NewSessionCommonInfo(msg.IsCompressed, method, key)

	connectTCP := true
	addr := msg.Addr
	mode := TunnelMode(msg.TunnelType)
	if mode == TunnelModeUDP {
		connectTCP = false
	}
	if mode.UsesRemoteUDPAddr() && !msg.IsTCP {
		connectTCP = false
		addr = ""
	}

	if connectTCP {
		dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
		conn, err := dialer.Dial("tcp", addr)
		if err != nil {
			o.output(O2IConnect{TunnelID: msg.TunnelID, ID: msg.ID, Success: false, ErrorInfo: fmt.Sprintf("target=tcp://%s, reason=%v", addr, err)})
			return
		}
		if tcpConn, ok := conn.(*net.TCPConn); ok {
			if err := configureTCPConn(tcpConn); err != nil {
				_ = conn.Close()
				o.output(O2IConnect{TunnelID: msg.TunnelID, ID: msg.ID, Success: false, ErrorInfo: err.Error()})
				return
			}
		}
		writer := NewTCPWriter(conn)
		o.putSession(msg.ID, &outletSession{id: msg.ID, writer: writer, common: common, close: func() { _ = conn.Close() }, inputQ: newProxyMessageQueue()})
		o.output(O2IConnect{TunnelID: msg.TunnelID, ID: msg.ID, Success: true})
		safeGo(o.logger, goroutineName("outlet-read-tcp-", msg.ID), func() {
			o.readTCP(msg.TunnelID, msg.ID, conn, common, mode == TunnelModeSOCKS5)
		})
		return
	}

	localAddr := &net.UDPAddr{IP: net.IPv4zero, Port: 0}
	var conn *net.UDPConn
	if addr != "" {
		remote, err := net.ResolveUDPAddr("udp", addr)
		if err != nil {
			o.output(O2IConnect{TunnelID: msg.TunnelID, ID: msg.ID, Success: false, ErrorInfo: err.Error()})
			return
		}
		conn, err = net.DialUDP("udp", localAddr, remote)
		if err != nil {
			o.output(O2IConnect{TunnelID: msg.TunnelID, ID: msg.ID, Success: false, ErrorInfo: err.Error()})
			return
		}
	} else {
		conn, err = net.ListenUDP("udp", localAddr)
		if err != nil {
			o.output(O2IConnect{TunnelID: msg.TunnelID, ID: msg.ID, Success: false, ErrorInfo: err.Error()})
			return
		}
	}
	writer := NewUDPWriter(conn, nil)
	o.putSession(msg.ID, &outletSession{id: msg.ID, writer: writer, common: common, close: func() { _ = conn.Close() }, inputQ: newProxyMessageQueue()})
	o.output(O2IConnect{TunnelID: msg.TunnelID, ID: msg.ID, Success: true})
	safeGo(o.logger, goroutineName("outlet-read-udp-", msg.ID), func() {
		o.readUDP(msg.TunnelID, msg.ID, conn, common, mode.UsesRemoteUDPAddr())
	})
}

func (o *Outlet) onSendData(msg I2OSendData) error {
	session, ok := o.session(msg.ID)
	if !ok {
		return nil
	}
	decoded, err := session.common.DecodeData(msg.Data)
	if err != nil {
		return err
	}
	return session.writer.Write(decoded, func() {
		o.output(O2ISendDataResult{TunnelID: msg.TunnelID, ID: msg.ID, DataLen: uint32(len(msg.Data))})
	})
}

func (o *Outlet) onSendToData(msg I2OSendToData) error {
	session, ok := o.session(msg.ID)
	if !ok {
		return nil
	}
	decoded, err := session.common.DecodeData(msg.Data)
	if err != nil {
		return err
	}
	target, err := net.ResolveUDPAddr("udp", msg.TargetAddr)
	if err != nil {
		return err
	}
	if err := session.writer.WriteTo(decoded, target); err != nil {
		return err
	}
	o.output(O2ISendDataResult{TunnelID: msg.TunnelID, ID: msg.ID, DataLen: uint32(len(msg.Data))})
	return nil
}

func (o *Outlet) onDisconnect(msg I2ODisconnect) error {
	session, ok := o.session(msg.ID)
	if !ok {
		return nil
	}
	session.common.Close()
	o.removeSession(msg.ID)
	if session.close != nil {
		session.close()
	}
	return nil
}

func (o *Outlet) onRecvResult(msg I2ORecvDataResult) {
	session, ok := o.session(msg.ID)
	if !ok {
		return
	}
	session.common.Flow.Release(int(msg.DataLen))
}

func (o *Outlet) readTCP(tunnelID, sessionID uint32, conn net.Conn, common *SessionCommonInfo, _ bool) {
	defer func() {
		o.removeSession(sessionID)
		o.output(O2IDisconnect{TunnelID: tunnelID, ID: sessionID})
		_ = conn.Close()
	}()
	buf := make([]byte, 65535)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			return
		}
		encoded, err := common.EncodeDataAndLimit(append([]byte(nil), buf[:n]...))
		if err != nil {
			o.logger.Printf("出口 TCP 编码失败: %v", err)
			return
		}
		// 中文注释：SOCKS5 的 TCP CONNECT 返回的是纯 TCP 字节流，
		// 必须对齐 Rust 基线走 O2IRecvData，只有 UDP ASSOCIATE 才走 O2IRecvDataFrom。
		o.output(O2IRecvData{TunnelID: tunnelID, ID: sessionID, Data: encoded})
	}
}

func (o *Outlet) readUDP(tunnelID, sessionID uint32, conn *net.UDPConn, common *SessionCommonInfo, socks5 bool) {
	defer func() {
		o.removeSession(sessionID)
		o.output(O2IDisconnect{TunnelID: tunnelID, ID: sessionID})
		_ = conn.Close()
	}()
	buf := make([]byte, 65535)
	for {
		n, addr, err := conn.ReadFromUDP(buf)
		if err != nil {
			return
		}
		encoded, err := common.EncodeDataAndLimit(append([]byte(nil), buf[:n]...))
		if err != nil {
			o.logger.Printf("出口 UDP 编码失败: %v", err)
			return
		}
		if socks5 {
			o.output(O2IRecvDataFrom{TunnelID: tunnelID, ID: sessionID, Data: encoded, RemoteAddr: addr.String()})
		} else {
			o.output(O2IRecvData{TunnelID: tunnelID, ID: sessionID, Data: encoded})
		}
	}
}

func (o *Outlet) session(id uint32) (*outletSession, bool) {
	o.mu.RLock()
	defer o.mu.RUnlock()
	session, ok := o.sessions[id]
	return session, ok
}

func (o *Outlet) putSession(id uint32, session *outletSession) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.sessions[id] = session
	safeGo(o.logger, goroutineName("outlet-session-", id), func() {
		for {
			message, ok := session.inputQ.Pop()
			if !ok {
				return
			}
			switch msg := message.(type) {
			case I2OSendData:
				if err := o.onSendData(msg); err != nil {
					o.logger.Printf("出口发送数据失败: %v", err)
				}
			case I2OSendToData:
				if err := o.onSendToData(msg); err != nil {
					o.logger.Printf("出口发送目标数据失败: %v", err)
				}
			case I2ODisconnect:
				if err := o.onDisconnect(msg); err != nil {
					o.logger.Printf("出口断开失败: %v", err)
				}
			}
		}
	})
}

func (o *Outlet) removeSession(id uint32) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if session, ok := o.sessions[id]; ok {
		session.common.Close()
		session.stopInput()
		delete(o.sessions, id)
	}
}

func (s *outletSession) stopInput() {
	s.once.Do(func() {
		s.inputQ.Close()
	})
}
