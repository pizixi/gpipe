package proxy

import (
	"fmt"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

const (
	sessionReadyTimeout = 10 * time.Second
	udpSessionIdleTTL   = 10 * time.Second
)

type inletSession struct {
	id      uint32
	writer  PeerWriter
	context ContextHandler
	data    *ContextData
	close   func()
	inputQ  *proxyMessageQueue
	peerQ   *byteQueue
	peerKey string

	ctxMu     sync.Mutex
	readyCh   chan struct{}
	readyOnce sync.Once
	closed    chan struct{}
	closeOnce sync.Once
	lastSeen  atomic.Int64
}

// Inlet 负责接收本地入口流量，并把数据转换成代理消息。
type Inlet struct {
	logger      *log.Logger
	tunnelID    uint32
	mode        TunnelMode
	listenAddr  string
	outputAddr  string
	authData    InletAuthData
	output      OutputFunc
	common      *SessionCommonInfo
	description string

	mu       sync.RWMutex
	sessions map[uint32]*inletSession
	udpPeers map[string]uint32
	stopCh   chan struct{}
	stopOnce sync.Once
	runWG    sync.WaitGroup
}

func NewInlet(logger *log.Logger, tunnelID uint32, mode TunnelMode, listenAddr, outputAddr string, common *SessionCommonInfo, authData InletAuthData, output OutputFunc, description string) *Inlet {
	return &Inlet{
		logger:      logger,
		tunnelID:    tunnelID,
		mode:        mode,
		listenAddr:  listenAddr,
		outputAddr:  outputAddr,
		authData:    authData,
		output:      output,
		common:      common,
		description: description,
		sessions:    map[uint32]*inletSession{},
		udpPeers:    map[string]uint32{},
		stopCh:      make(chan struct{}),
	}
}

func (i *Inlet) Description() string {
	return i.description
}

func (i *Inlet) Start() error {
	switch i.mode {
	case TunnelModeTCP, TunnelModeSOCKS5, TunnelModeHTTP:
		return i.startTCP()
	case TunnelModeUDP:
		return i.startUDP()
	case TunnelModeShadowsocks:
		if err := i.startTCP(); err != nil {
			return err
		}
		if err := i.startUDP(); err != nil {
			_ = i.Stop()
			return err
		}
		return nil
	default:
		return fmt.Errorf("未知入口类型")
	}
}

func (i *Inlet) Stop() error {
	i.stopOnce.Do(func() {
		close(i.stopCh)
	})

	i.mu.Lock()
	sessions := make([]*inletSession, 0, len(i.sessions))
	for _, session := range i.sessions {
		sessions = append(sessions, session)
	}
	i.sessions = map[uint32]*inletSession{}
	i.udpPeers = map[string]uint32{}
	i.mu.Unlock()

	for _, session := range sessions {
		session.shutdown(i.logger)
	}
	i.runWG.Wait()
	return nil
}

func (i *Inlet) runAsync(name string, fn func()) {
	i.runWG.Add(1)
	safeGo(i.logger, name, func() {
		defer i.runWG.Done()
		fn()
	})
}

func (i *Inlet) Input(message ProxyMessage) {
	i.mu.RLock()
	session := i.sessions[message.SessionID()]
	i.mu.RUnlock()
	if session == nil {
		return
	}
	switch msg := message.(type) {
	case O2ISendDataResult:
		session.data.common.Flow.Release(int(msg.DataLen))
	case O2IConnect, O2IRecvData, O2IRecvDataFrom, O2IDisconnect:
		if !session.inputQ.Push(message) {
			i.logger.Printf("入口会话消息被丢弃: tunnel=%d session=%d", i.tunnelID, message.SessionID())
			i.closeSession(message.SessionID())
		}
	}
}

func (i *Inlet) startTCP() error {
	ln, err := net.Listen("tcp", i.listenAddr)
	if err != nil {
		return err
	}
	i.runAsync(goroutineName("inlet-stop-tcp-listener-", i.tunnelID), func() {
		<-i.stopCh
		_ = ln.Close()
	})
	i.runAsync(goroutineName("inlet-accept-", i.tunnelID), func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				if isExpectedNetCloseError(err) {
					return
				}
				select {
				case <-i.stopCh:
					return
				default:
				}
				i.logger.Printf("入口接受 TCP 连接失败: %v", err)
				continue
			}
			if tcpConn, ok := conn.(*net.TCPConn); ok {
				if err := configureTCPConn(tcpConn); err != nil {
					i.logger.Printf("入口配置 TCP 连接失败: %v", err)
					_ = conn.Close()
					continue
				}
			}
			i.runAsync(goroutineName("inlet-tcp-session-", conn.RemoteAddr()), func() {
				i.runTCPConn(conn)
			})
		}
	})
	return nil
}

func (i *Inlet) startUDP() error {
	addr, err := net.ResolveUDPAddr("udp", i.listenAddr)
	if err != nil {
		return err
	}
	socket, err := net.ListenUDP("udp", addr)
	if err != nil {
		return err
	}
	i.runAsync(goroutineName("inlet-stop-udp-socket-", i.tunnelID), func() {
		<-i.stopCh
		_ = socket.Close()
	})
	i.runAsync(goroutineName("inlet-udp-loop-", i.tunnelID), func() {
		buf := make([]byte, 65535)
		for {
			n, peer, err := socket.ReadFromUDP(buf)
			if err != nil {
				if isExpectedNetCloseError(err) {
					return
				}
				select {
				case <-i.stopCh:
					return
				default:
				}
				i.logger.Printf("入口 UDP 收包失败: %v", err)
				continue
			}
			payload := append([]byte(nil), buf[:n]...)
			session, created, err := i.ensureUDPSession(socket, peer)
			if err != nil {
				i.logger.Printf("入口 UDP 会话创建失败: %v", err)
				continue
			}
			if session == nil {
				continue
			}
			session.touch()
			if !session.peerQ.Push(payload) {
				i.logger.Printf("入口 UDP 数据包被丢弃: tunnel=%d peer=%s", i.tunnelID, peer)
				if created {
					i.closeSession(session.id)
				}
			}
		}
	})
	return nil
}

func (i *Inlet) runTCPConn(conn net.Conn) {
	sessionID := NextSessionID()
	writer := NewTCPWriter(conn)
	peer := conn.RemoteAddr()
	contextFactory := i.contextFactory()
	session, err := i.runSession(sessionID, writer, peer, contextFactory, func() { _ = conn.Close() }, "")
	if err != nil {
		i.logger.Printf("入口 TCP 会话启动失败: %v", err)
		_ = conn.Close()
		return
	}
	if !session.waitReady(sessionReadyTimeout) {
		i.closeSession(sessionID)
		return
	}

	buf := make([]byte, 65535)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			if !isExpectedNetCloseError(err) {
				i.logger.Printf("入口 TCP 读取失败: %v", err)
			}
			i.closeSession(sessionID)
			return
		}
		if err := session.handlePeerData(append([]byte(nil), buf[:n]...)); err != nil {
			if !isExpectedNetCloseError(err) {
				i.logger.Printf("入口读取处理失败: %v", err)
			}
			i.closeSession(sessionID)
			return
		}
	}
}

func (i *Inlet) ensureUDPSession(socket *net.UDPConn, peer *net.UDPAddr) (*inletSession, bool, error) {
	peerKey := peer.String()

	i.mu.RLock()
	sessionID, ok := i.udpPeers[peerKey]
	var session *inletSession
	if ok {
		session = i.sessions[sessionID]
	}
	i.mu.RUnlock()
	if session != nil {
		return session, false, nil
	}

	sessionID = NextSessionID()
	writer := NewUDPWriter(socket, peer)
	session, err := i.runSession(sessionID, writer, peer, func() ContextHandler {
		return i.newUDPContext()
	}, nil, peerKey)
	if err != nil {
		return nil, false, err
	}
	i.runAsync(goroutineName("inlet-udp-session-", sessionID), func() {
		i.runUDPSession(session)
	})
	i.runAsync(goroutineName("inlet-udp-watch-", sessionID), func() {
		i.runUDPIdleWatcher(session)
	})
	return session, true, nil
}

func (i *Inlet) runSession(sessionID uint32, writer PeerWriter, peer net.Addr, contextFactory func() ContextHandler, closeFn func(), peerKey string) (*inletSession, error) {
	common := i.common.Clone()
	data := NewContextData(i.tunnelID, i.mode, i.outputAddr, i.output, common, i.authData)
	data.SetSessionID(sessionID)

	session := &inletSession{
		id:      sessionID,
		writer:  writer,
		context: contextFactory(),
		data:    data,
		close:   closeFn,
		inputQ:  newProxyMessageQueue(),
		peerKey: peerKey,
		readyCh: make(chan struct{}),
		closed:  make(chan struct{}),
	}
	if peerKey != "" {
		session.peerQ = newByteQueue()
	}
	session.touch()

	i.mu.Lock()
	if _, exists := i.sessions[sessionID]; exists {
		i.mu.Unlock()
		return nil, fmt.Errorf("duplicate session id: %d", sessionID)
	}
	i.sessions[sessionID] = session
	if peerKey != "" {
		i.udpPeers[peerKey] = sessionID
	}
	i.mu.Unlock()

	i.runAsync(goroutineName("inlet-proxy-session-", sessionID), func() {
		i.runProxyMessageLoop(session)
	})

	if err := session.onStart(peer, writer); err != nil {
		i.closeSession(sessionID)
		return nil, err
	}
	return session, nil
}

func (i *Inlet) runProxyMessageLoop(session *inletSession) {
	for {
		message, ok := session.inputQ.Pop()
		if !ok {
			return
		}
		shouldClose, err := session.handleProxyMessage(message)
		if err != nil {
			if !isExpectedNetCloseError(err) {
				i.logger.Printf("入口会话处理代理消息失败: %v", err)
			}
			i.closeSession(session.id)
			return
		}
		if shouldClose {
			i.closeSession(session.id)
			return
		}
	}
}

func (i *Inlet) runUDPSession(session *inletSession) {
	for {
		payload, ok := session.peerQ.Pop()
		if !ok {
			return
		}
		if !session.ready() && !session.waitReady(sessionReadyTimeout) {
			i.closeSession(session.id)
			return
		}
		if err := session.handlePeerData(payload); err != nil {
			if !isExpectedNetCloseError(err) {
				i.logger.Printf("入口 UDP 处理失败: %v", err)
			}
			i.closeSession(session.id)
			return
		}
	}
}

func (i *Inlet) runUDPIdleWatcher(session *inletSession) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-session.closed:
			return
		case <-ticker.C:
			if session.idleFor() > udpSessionIdleTTL {
				i.closeSession(session.id)
				return
			}
		}
	}
}

func (i *Inlet) contextFactory() func() ContextHandler {
	switch i.mode {
	case TunnelModeSOCKS5:
		return func() ContextHandler { return NewSocks5Context() }
	case TunnelModeHTTP:
		return func() ContextHandler { return NewHTTPContext() }
	case TunnelModeShadowsocks:
		return func() ContextHandler { return NewShadowsocksTCPContext() }
	default:
		return func() ContextHandler { return NewUniversalContext() }
	}
}

func (i *Inlet) newUDPContext() ContextHandler {
	if i.mode == TunnelModeShadowsocks {
		return NewShadowsocksUDPContext()
	}
	return NewUniversalContext()
}

func (i *Inlet) closeSession(sessionID uint32) {
	session := i.removeSession(sessionID)
	if session == nil {
		return
	}
	session.shutdown(i.logger)
}

func (i *Inlet) removeSession(sessionID uint32) *inletSession {
	i.mu.Lock()
	defer i.mu.Unlock()
	session := i.sessions[sessionID]
	if session == nil {
		return nil
	}
	delete(i.sessions, sessionID)
	if session.peerKey != "" {
		if mappedID, ok := i.udpPeers[session.peerKey]; ok && mappedID == sessionID {
			delete(i.udpPeers, session.peerKey)
		}
	}
	return session
}

func (i *Inlet) session(sessionID uint32) *inletSession {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.sessions[sessionID]
}

func (s *inletSession) onStart(peer net.Addr, writer PeerWriter) error {
	s.ctxMu.Lock()
	defer s.ctxMu.Unlock()
	if err := s.context.OnStart(s.data, peer, writer); err != nil {
		return err
	}
	s.markReadyLocked()
	return nil
}

func (s *inletSession) handlePeerData(payload []byte) error {
	s.touch()
	s.ctxMu.Lock()
	defer s.ctxMu.Unlock()
	if err := s.context.OnPeerData(s.data, payload); err != nil {
		return err
	}
	s.markReadyLocked()
	return nil
}

func (s *inletSession) handleProxyMessage(message ProxyMessage) (bool, error) {
	s.touch()
	s.ctxMu.Lock()
	err := s.context.OnProxyMessage(message)
	s.markReadyLocked()
	s.ctxMu.Unlock()
	if err != nil {
		return true, err
	}
	switch msg := message.(type) {
	case O2IConnect:
		if msg.Success {
			return false, nil
		}
		// 中文注释：HTTP/SOCKS5 在建连失败时会先把错误响应写回本地客户端，
		// 然后由上下文自己安排延迟关闭，不能在这里立即回收会话。
		switch s.data.mode {
		case TunnelModeHTTP, TunnelModeSOCKS5:
			return false, nil
		default:
			return true, nil
		}
	case O2IDisconnect:
		return true, nil
	default:
		return false, nil
	}
}

func (s *inletSession) shutdown(logger *log.Logger) {
	s.closeOnce.Do(func() {
		close(s.closed)
		s.data.common.Close()
		s.inputQ.Close()
		if s.peerQ != nil {
			s.peerQ.Close()
		}
		s.ctxMu.Lock()
		err := s.context.OnStop(s.data)
		s.ctxMu.Unlock()
		if err != nil && logger != nil {
			logger.Printf("入口会话关闭清理失败: session=%d err=%v", s.id, err)
		}
		safeClose(logger, goroutineName("inlet-session-", s.id), func() error {
			if s.close != nil {
				s.close()
			}
			return nil
		})
	})
}

func (s *inletSession) markReadyLocked() {
	if s.context.ReadyForRead() {
		s.readyOnce.Do(func() {
			close(s.readyCh)
		})
	}
}

func (s *inletSession) ready() bool {
	select {
	case <-s.readyCh:
		return true
	default:
		return false
	}
}

func (s *inletSession) waitReady(timeout time.Duration) bool {
	if s.ready() {
		return true
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-s.readyCh:
		return true
	case <-s.closed:
		return false
	case <-timer.C:
		return false
	}
}

func (s *inletSession) touch() {
	s.lastSeen.Store(time.Now().UnixNano())
}

func (s *inletSession) idleFor() time.Duration {
	last := s.lastSeen.Load()
	if last == 0 {
		return 0
	}
	return time.Since(time.Unix(0, last))
}
