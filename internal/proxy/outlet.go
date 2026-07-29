package proxy

import (
	"context"
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

type outletConnectAttempt struct {
	cancel context.CancelFunc
}

// Outlet 负责连接出口目标，并把远端返回的数据转成代理消息。
type Outlet struct {
	logger      *log.Logger
	tunnelID    uint32
	description string
	output      OutputFunc
	dialContext func(context.Context, string, string) (net.Conn, error)

	mu         sync.RWMutex
	sessions   map[uint32]*outletSession
	connecting map[uint32]*outletConnectAttempt
	stopped    bool
	connectWG  sync.WaitGroup
}

func NewOutlet(logger *log.Logger, output OutputFunc, description string) *Outlet {
	dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	return &Outlet{
		logger:      logger,
		description: description,
		output:      output,
		sessions:    map[uint32]*outletSession{},
		connecting:  map[uint32]*outletConnectAttempt{},
		dialContext: dialer.DialContext,
	}
}

func (o *Outlet) Description() string {
	return o.description
}

func (o *Outlet) Stop() error {
	o.mu.Lock()
	if o.stopped {
		o.mu.Unlock()
		o.connectWG.Wait()
		return nil
	}
	o.stopped = true
	cancels := make([]context.CancelFunc, 0, len(o.connecting))
	for _, attempt := range o.connecting {
		cancels = append(cancels, attempt.cancel)
	}
	o.connecting = map[uint32]*outletConnectAttempt{}
	sessions := make([]*outletSession, 0, len(o.sessions))
	for _, session := range o.sessions {
		sessions = append(sessions, session)
	}
	o.sessions = map[uint32]*outletSession{}
	o.mu.Unlock()

	for _, cancel := range cancels {
		cancel()
	}
	for _, session := range sessions {
		session.common.Close()
		session.stopInput()
		if session.close != nil {
			session.close()
		}
	}
	o.connectWG.Wait()
	return nil
}

func (o *Outlet) Input(message ProxyMessage) {
	switch msg := message.(type) {
	case I2OConnect:
		o.startConnect(msg)
	case I2OSendData:
		if session, ok := o.session(msg.ID); ok {
			if !session.inputQ.Push(msg) {
				o.logger.Printf("出口会话消息被丢弃: session=%d", msg.ID)
				o.terminateSession(msg.TunnelID, msg.ID)
			}
		}
	case I2OSendToData:
		if session, ok := o.session(msg.ID); ok {
			if !session.inputQ.Push(msg) {
				o.logger.Printf("出口会话消息被丢弃: session=%d", msg.ID)
				o.terminateSession(msg.TunnelID, msg.ID)
			}
		}
	case I2ODisconnect:
		o.onDisconnectInput(msg)
	case I2ORecvDataResult:
		o.onRecvResult(msg)
	}
}

func (o *Outlet) startConnect(msg I2OConnect) {
	ctx, cancel := context.WithCancel(context.Background())
	attempt := &outletConnectAttempt{cancel: cancel}
	o.mu.Lock()
	if o.stopped {
		o.mu.Unlock()
		cancel()
		return
	}
	if _, ok := o.sessions[msg.ID]; ok {
		o.mu.Unlock()
		cancel()
		o.output(O2IConnect{TunnelID: msg.TunnelID, ID: msg.ID, Success: false, ErrorInfo: "repeated connection"})
		return
	}
	if _, ok := o.connecting[msg.ID]; ok {
		o.mu.Unlock()
		cancel()
		o.output(O2IConnect{TunnelID: msg.TunnelID, ID: msg.ID, Success: false, ErrorInfo: "repeated connection"})
		return
	}
	o.connecting[msg.ID] = attempt
	o.connectWG.Add(1)
	o.mu.Unlock()

	safeGo(o.logger, goroutineName("outlet-connect-", msg.ID), func() {
		defer o.connectWG.Done()
		defer o.finishConnectAttempt(msg.ID, attempt)
		o.onConnect(ctx, msg, attempt)
	})
}

func (o *Outlet) onConnect(ctx context.Context, msg I2OConnect, attempt *outletConnectAttempt) {

	method := ParseEncryptionMethod(msg.EncryptionMethod)
	key, err := DecodeKeyFromBase64(msg.EncryptionKey)
	if err != nil {
		o.failConnect(msg, attempt, err.Error())
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
		conn, err := o.dialContext(ctx, "tcp", addr)
		if err != nil {
			o.failConnect(msg, attempt, fmt.Sprintf("target=tcp://%s, reason=%v", addr, err))
			return
		}
		if tcpConn, ok := conn.(*net.TCPConn); ok {
			if err := configureTCPConn(tcpConn); err != nil {
				_ = conn.Close()
				o.failConnect(msg, attempt, err.Error())
				return
			}
		}
		writer := NewTCPWriter(conn)
		session := &outletSession{id: msg.ID, writer: writer, common: common, close: func() { _ = conn.Close() }, inputQ: newProxyMessageQueue()}
		if !o.installConnectedSession(msg.ID, attempt, session) {
			common.Close()
			_ = conn.Close()
			return
		}
		o.output(O2IConnect{TunnelID: msg.TunnelID, ID: msg.ID, Success: true})
		safeGo(o.logger, goroutineName("outlet-read-tcp-", msg.ID), func() {
			o.readTCP(msg.TunnelID, msg.ID, conn, common, mode == TunnelModeSOCKS5)
		})
		return
	}

	localAddr := &net.UDPAddr{IP: net.IPv4zero, Port: 0}
	var conn *net.UDPConn
	if addr != "" {
		rawConn, dialErr := o.dialContext(ctx, "udp", addr)
		err = dialErr
		if err != nil {
			o.failConnect(msg, attempt, err.Error())
			return
		}
		var ok bool
		conn, ok = rawConn.(*net.UDPConn)
		if !ok {
			_ = rawConn.Close()
			o.failConnect(msg, attempt, "udp dial did not return a UDP connection")
			return
		}
	} else {
		conn, err = net.ListenUDP("udp", localAddr)
		if err != nil {
			o.failConnect(msg, attempt, err.Error())
			return
		}
	}
	writer := NewUDPWriter(conn, nil)
	session := &outletSession{id: msg.ID, writer: writer, common: common, close: func() { _ = conn.Close() }, inputQ: newProxyMessageQueue()}
	if !o.installConnectedSession(msg.ID, attempt, session) {
		common.Close()
		_ = conn.Close()
		return
	}
	o.output(O2IConnect{TunnelID: msg.TunnelID, ID: msg.ID, Success: true})
	safeGo(o.logger, goroutineName("outlet-read-udp-", msg.ID), func() {
		o.readUDP(msg.TunnelID, msg.ID, conn, common, mode.UsesRemoteUDPAddr())
	})
}

func (o *Outlet) failConnect(msg I2OConnect, attempt *outletConnectAttempt, message string) {
	o.mu.RLock()
	active := o.connecting[msg.ID] == attempt
	stopped := o.stopped
	o.mu.RUnlock()
	if active && !stopped {
		o.output(O2IConnect{TunnelID: msg.TunnelID, ID: msg.ID, Success: false, ErrorInfo: message})
	}
}

func (o *Outlet) finishConnectAttempt(id uint32, attempt *outletConnectAttempt) {
	o.mu.Lock()
	if o.connecting[id] == attempt {
		delete(o.connecting, id)
	}
	o.mu.Unlock()
	attempt.cancel()
}

func (o *Outlet) installConnectedSession(id uint32, attempt *outletConnectAttempt, session *outletSession) bool {
	o.mu.Lock()
	if o.stopped || o.connecting[id] != attempt || o.sessions[id] != nil {
		o.mu.Unlock()
		return false
	}
	delete(o.connecting, id)
	o.sessions[id] = session
	o.mu.Unlock()
	attempt.cancel()
	o.startSessionInput(id, session)
	return true
}

// onDisconnectInput 在同一把锁下判断“拨号中”与“已连接”，避免断开消息
// 落在连接安装的切换窗口内而遗留无人管理的目标连接。
func (o *Outlet) onDisconnectInput(msg I2ODisconnect) {
	o.mu.Lock()
	session := o.sessions[msg.ID]
	attempt := o.connecting[msg.ID]
	if attempt != nil {
		delete(o.connecting, msg.ID)
	}
	o.mu.Unlock()

	if attempt != nil {
		attempt.cancel()
	}
	if session == nil {
		return
	}
	if !session.inputQ.Push(msg) {
		o.logger.Printf("出口会话消息被丢弃: session=%d", msg.ID)
		o.terminateSession(msg.TunnelID, msg.ID)
	}
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
	session := o.detachSession(msg.ID)
	if session == nil {
		return nil
	}
	safeClose(o.logger, goroutineName("outlet-session-", msg.ID), func() error {
		if session.close != nil {
			session.close()
		}
		return nil
	})
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
	buf := make([]byte, proxyTCPReadBufferSize)
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
	buf := make([]byte, proxyUDPReadBufferSize)
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

func (o *Outlet) putSession(id uint32, session *outletSession) bool {
	o.mu.Lock()
	if o.stopped || o.sessions[id] != nil {
		o.mu.Unlock()
		return false
	}
	o.sessions[id] = session
	o.mu.Unlock()
	o.startSessionInput(id, session)
	return true
}

func (o *Outlet) startSessionInput(id uint32, session *outletSession) {
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
					o.terminateSession(msg.TunnelID, msg.ID)
					return
				}
			case I2OSendToData:
				if err := o.onSendToData(msg); err != nil {
					o.logger.Printf("出口发送目标数据失败: %v", err)
					o.terminateSession(msg.TunnelID, msg.ID)
					return
				}
			case I2ODisconnect:
				if err := o.onDisconnect(msg); err != nil {
					o.logger.Printf("出口断开失败: %v", err)
				}
				return
			}
		}
	})
}

func (o *Outlet) removeSession(id uint32) {
	_ = o.detachSession(id)
}

func (o *Outlet) detachSession(id uint32) *outletSession {
	o.mu.Lock()
	defer o.mu.Unlock()
	session := o.sessions[id]
	if session == nil {
		return nil
	}
	session.common.Close()
	session.stopInput()
	delete(o.sessions, id)
	return session
}

func (o *Outlet) terminateSession(tunnelID, sessionID uint32) {
	session := o.detachSession(sessionID)
	if session == nil {
		return
	}
	safeClose(o.logger, goroutineName("outlet-session-", sessionID), func() error {
		if session.close != nil {
			session.close()
		}
		return nil
	})
	o.output(O2IDisconnect{TunnelID: tunnelID, ID: sessionID})
}

func (s *outletSession) stopInput() {
	s.once.Do(func() {
		s.inputQ.Close()
	})
}
