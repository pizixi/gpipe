package server

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pizixi/gpipe/internal/codec"
	"github.com/pizixi/gpipe/internal/manager"
	"github.com/pizixi/gpipe/internal/model"
	"github.com/pizixi/gpipe/internal/pb"
	"github.com/pizixi/gpipe/internal/proto"
	"github.com/pizixi/gpipe/internal/proxy"
)

const (
	sessionReadTimeout        = 45 * time.Second
	sessionWriteTimeout       = 15 * time.Second
	sessionWriteQueueCapacity = 1024
	sessionReadCacheCompactAt = 256 * 1024
)

var errSessionWriteQueueFull = errors.New("session write queue full")

type Hub struct {
	logger         *log.Logger
	runtime        *manager.Runtime
	proxyMgr       *proxy.Manager
	illegalForward string
	sessions       sync.Map
	nextID         atomic.Uint32
}

func NewHub(logger *log.Logger) *Hub {
	h := &Hub{logger: logger}
	h.nextID.Store(1)
	return h
}

func (h *Hub) SetRuntime(rt *manager.Runtime) {
	h.runtime = rt
	h.proxyMgr = proxy.NewManager(h.logger, 0, func(playerID uint32, message any) error {
		session := h.sessionFor(playerID)
		if session == nil {
			return fmt.Errorf("player %d offline", playerID)
		}
		return session.SendPush(message)
	})
	h.proxyMgr.SetRuntimeReporter(func(event proxy.TunnelRuntimeEvent) {
		if h.runtime == nil || h.runtime.TunnelRuntime == nil {
			return
		}
		switch event.Component {
		case proxy.RuntimeComponentInlet:
			h.runtime.TunnelRuntime.SetInlet(event.TunnelID, event.Running, event.Error)
		case proxy.RuntimeComponentOutlet:
			h.runtime.TunnelRuntime.SetOutlet(event.TunnelID, event.Running, event.Error)
		}
	})
	var tunnels []*pb.Tunnel
	for _, tunnel := range rt.Tunnel.All() {
		if tunnel.Sender == 0 || tunnel.Receiver == 0 {
			tunnels = append(tunnels, h.runtimeTunnelPB(tunnel))
		}
	}
	h.proxyMgr.SyncTunnels(tunnels)
}

func (h *Hub) NewSession(conn net.Conn) *Session {
	return &Session{
		id:         h.nextID.Add(1),
		conn:       conn,
		hub:        h,
		logger:     h.logger,
		writeQueue: newWriteQueue(),
		closeCh:    make(chan struct{}),
	}
}

func (h *Hub) registerPlayer(playerID uint32, session *Session) {
	h.sessions.Store(playerID, session)
}

func (h *Hub) unregisterPlayer(playerID uint32, session *Session) {
	loaded, ok := h.sessions.Load(playerID)
	if ok && loaded == session {
		h.sessions.Delete(playerID)
	}
}

func (h *Hub) sessionFor(playerID uint32) *Session {
	value, ok := h.sessions.Load(playerID)
	if !ok {
		return nil
	}
	session, _ := value.(*Session)
	return session
}

func (h *Hub) BroadcastTunnel(playerID uint32, tunnel model.Tunnel, isDelete bool) {
	update := h.runtimeTunnelUpdate(tunnel, isDelete)
	if playerID == 0 {
		h.syncServerTunnelUpdate(update)
		return
	}
	h.pushTunnelUpdate(playerID, update)
}

func (h *Hub) pushTunnelUpdate(playerID uint32, update *pb.ModifyTunnelNtf) {
	if playerID == 0 || update == nil {
		return
	}
	session := h.sessionFor(playerID)
	if session == nil {
		return
	}
	if err := session.SendPush(update); err != nil {
		h.logger.Printf("broadcast tunnel to player %d failed: %v", playerID, err)
		_ = session.Close()
	}
}

func (h *Hub) syncServerTunnelUpdate(update *pb.ModifyTunnelNtf) {
	if update == nil || update.Tunnel == nil || h.proxyMgr == nil {
		return
	}
	if update.Tunnel.Sender == 0 || update.Tunnel.Receiver == 0 {
		h.proxyMgr.UpdateTunnel(update)
	}
}

func (h *Hub) broadcastPlayerTunnelStates(playerID, skipPlayerID uint32) {
	if h.runtime == nil || playerID == 0 {
		return
	}
	for _, tunnel := range h.runtime.Tunnel.ByPlayer(playerID) {
		update := h.runtimeTunnelUpdate(tunnel, false)
		h.syncServerTunnelUpdate(update)
		if tunnel.Sender != 0 && tunnel.Sender != skipPlayerID {
			h.pushTunnelUpdate(tunnel.Sender, update)
		}
		if tunnel.Receiver != 0 && tunnel.Receiver != skipPlayerID && tunnel.Receiver != tunnel.Sender {
			h.pushTunnelUpdate(tunnel.Receiver, update)
		}
	}
}

func (h *Hub) playerAvailable(playerID uint32) bool {
	if playerID == 0 {
		return true
	}
	return h.runtime != nil && h.runtime.Players.IsOnline(playerID)
}

func (h *Hub) tunnelRuntimeEnabled(tunnel model.Tunnel) bool {
	return tunnel.Enabled && h.playerAvailable(tunnel.Sender) && h.playerAvailable(tunnel.Receiver)
}

type Session struct {
	id         uint32
	conn       net.Conn
	hub        *Hub
	logger     *log.Logger
	playerID   uint32
	writeQueue *writeQueue
	closeCh    chan struct{}
	once       sync.Once
}

func (s *Session) Run() {
	go s.writeLoop()
	s.readLoop()
	s.Close()
}

func (s *Session) SendPush(message proto.Message) error {
	return s.enqueue(0, message)
}

func (s *Session) Close() error {
	s.once.Do(func() {
		close(s.closeCh)
		s.writeQueue.Close()
		if s.playerID != 0 {
			playerID := s.playerID
			s.hub.unregisterPlayer(playerID, s)
			if s.hub.runtime != nil {
				s.hub.runtime.Players.Unbind(playerID, s)
				s.hub.broadcastPlayerTunnelStates(playerID, playerID)
			}
		}
		if s.conn != nil {
			_ = s.conn.Close()
		}
	})
	return nil
}

func (s *Session) writeLoop() {
	for {
		data, ok := s.writeQueue.Pop()
		if !ok {
			return
		}
		if len(data) == 0 {
			continue
		}
		if err := s.conn.SetWriteDeadline(time.Now().Add(sessionWriteTimeout)); err != nil {
			s.logger.Printf("session %d set write deadline error: %v", s.id, err)
			_ = s.Close()
			return
		}
		if err := writeAllToConn(s.conn, data); err != nil {
			if !isExpectedSessionCloseError(err) {
				s.logger.Printf("session %d write error: %v", s.id, err)
			}
			_ = s.Close()
			return
		}
	}
}

func (s *Session) readLoop() {
	buf := make([]byte, 32768)
	cache := make([]byte, 0, 64*1024)
	for {
		if err := s.conn.SetReadDeadline(time.Now().Add(sessionReadTimeout)); err != nil {
			s.logger.Printf("session %d set read deadline error: %v", s.id, err)
			return
		}
		n, err := s.conn.Read(buf)
		if err != nil {
			if err != io.EOF {
				s.logger.Printf("session %d read error: %v", s.id, err)
			}
			return
		}
		cache = append(cache, buf[:n]...)
		if len(cache) > 0 && cache[0] != 33 {
			if s.hub.illegalForward != "" {
				clearSessionDeadline(s.conn)
				s.forwardIllegalTraffic(cache)
				return
			}
			s.logger.Printf("session %d bad flag", s.id)
			return
		}
		for {
			frame, rest, err := codec.TryExtractFrame(cache, 2*1024*1024)
			if err != nil {
				s.logger.Printf("session %d decode frame error: %v", s.id, err)
				return
			}
			if frame == nil {
				cache = compactSessionReadCache(rest)
				break
			}
			cache = compactSessionReadCache(rest)
			if err := s.handleFrame(frame); err != nil {
				s.logger.Printf("session %d handle frame error: %v", s.id, err)
				return
			}
		}
	}
}

// forwardIllegalTraffic 对齐 Rust 的 illegal_traffic_forward 行为。
func (s *Session) forwardIllegalTraffic(initial []byte) {
	target, err := net.Dial("tcp", s.hub.illegalForward)
	if err != nil {
		s.logger.Printf("非法流量转发失败: %v", err)
		return
	}
	defer target.Close()
	if _, err := target.Write(initial); err != nil {
		return
	}
	go func() {
		_, _ = io.Copy(target, s.conn)
		_ = target.Close()
	}()
	_, _ = io.Copy(s.conn, target)
}

func (s *Session) handleFrame(frame []byte) error {
	serial, message, err := codec.Decode(frame)
	if err != nil {
		return err
	}
	switch {
	case serial < 0:
		response := s.handleRequest(message)
		if response != nil {
			return s.enqueue(-serial, response)
		}
	case serial == 0:
		return s.handlePush(message)
	default:
		return nil
	}
	return nil
}

func (s *Session) handleRequest(message proto.Message) proto.Message {
	switch msg := message.(type) {
	case *pb.Ping:
		return &pb.Pong{Ticks: msg.Ticks}
	case *pb.LoginReq:
		return s.onLogin(msg)
	case *pb.RegisterReq:
		return s.onRegister(msg)
	default:
		return &pb.Error{
			Number:  int32(pb.ErrorCodePlayerNotLogin),
			Message: "player not logged in",
		}
	}
}

func (s *Session) handlePush(message proto.Message) error {
	if s.playerID == 0 {
		return nil
	}
	if report, ok := message.(*pb.TunnelRuntimeReport); ok {
		s.handleTunnelRuntimeReport(report)
		return nil
	}
	tunnelID, fromPlayer, toPlayer, ok := s.resolveRoute(message)
	if !ok {
		return nil
	}
	target := s.hub.sessionFor(toPlayer)
	if target == nil {
		if toPlayer == 0 && s.hub.proxyMgr != nil {
			s.hub.proxyMgr.HandlePB(message)
			return nil
		}
		return s.handleOfflineProxy(message, tunnelID, fromPlayer)
	}
	if err := target.SendPush(message); err != nil {
		s.logger.Printf("session %d push to player %d failed: %v", s.id, toPlayer, err)
		_ = target.Close()
		return s.handleOfflineProxy(message, tunnelID, fromPlayer)
	}
	return nil
}

func (s *Session) handleTunnelRuntimeReport(report *pb.TunnelRuntimeReport) {
	if report == nil || report.TunnelID == 0 || s.hub.runtime == nil || s.hub.runtime.TunnelRuntime == nil {
		return
	}
	tunnel, ok := s.hub.runtime.Tunnel.Get(report.TunnelID)
	if !ok {
		return
	}
	switch report.Component {
	case string(proxy.RuntimeComponentInlet):
		if tunnel.Receiver != s.playerID {
			return
		}
		s.hub.runtime.TunnelRuntime.SetInlet(report.TunnelID, report.Running, report.Error)
	case string(proxy.RuntimeComponentOutlet):
		if tunnel.Sender != s.playerID {
			return
		}
		s.hub.runtime.TunnelRuntime.SetOutlet(report.TunnelID, report.Running, report.Error)
	default:
		return
	}
}

func (s *Session) resolveRoute(message proto.Message) (uint32, uint32, uint32, bool) {
	var tunnelID uint32
	isI2O := false
	switch msg := message.(type) {
	case *pb.I2OConnect:
		tunnelID, isI2O = msg.TunnelID, true
	case *pb.I2OSendData:
		tunnelID, isI2O = msg.TunnelID, true
	case *pb.I2OSendToData:
		tunnelID, isI2O = msg.TunnelID, true
	case *pb.I2ODisconnect:
		tunnelID, isI2O = msg.TunnelID, true
	case *pb.I2ORecvDataResult:
		tunnelID, isI2O = msg.TunnelID, true
	case *pb.O2IConnect:
		tunnelID = msg.TunnelID
	case *pb.O2IRecvData:
		tunnelID = msg.TunnelID
	case *pb.O2IRecvDataFrom:
		tunnelID = msg.TunnelID
	case *pb.O2IDisconnect:
		tunnelID = msg.TunnelID
	case *pb.O2ISendDataResult:
		tunnelID = msg.TunnelID
	default:
		return 0, 0, 0, false
	}
	tunnel, ok := s.hub.runtime.Tunnel.Get(tunnelID)
	if !ok {
		return 0, 0, 0, false
	}
	if isI2O {
		return tunnelID, tunnel.Receiver, tunnel.Sender, true
	}
	return tunnelID, tunnel.Sender, tunnel.Receiver, true
}

func (s *Session) handleOfflineProxy(message proto.Message, tunnelID, fromPlayer uint32) error {
	var reply proto.Message
	switch msg := message.(type) {
	case *pb.I2OConnect:
		reply = &pb.O2IConnect{
			TunnelID:  tunnelID,
			SessionID: msg.SessionID,
			Success:   false,
			ErrorInfo: fmt.Sprintf("no player for tunnel %d or the player is offline", tunnelID),
		}
	case *pb.I2OSendData:
		reply = &pb.O2IDisconnect{TunnelID: tunnelID, SessionID: msg.SessionID}
	case *pb.I2ORecvDataResult:
		reply = &pb.O2IDisconnect{TunnelID: tunnelID, SessionID: msg.SessionID}
	case *pb.O2IConnect:
		reply = &pb.I2ODisconnect{TunnelID: tunnelID, SessionID: msg.SessionID}
	case *pb.O2IRecvData:
		reply = &pb.I2ODisconnect{TunnelID: tunnelID, SessionID: msg.SessionID}
	case *pb.O2ISendDataResult:
		reply = &pb.I2ODisconnect{TunnelID: tunnelID, SessionID: msg.SessionID}
	}
	if reply == nil {
		return nil
	}
	source := s.hub.sessionFor(fromPlayer)
	if fromPlayer == 0 && s.hub.proxyMgr != nil {
		s.hub.proxyMgr.HandlePB(reply)
		return nil
	}
	if source == nil {
		return nil
	}
	return source.SendPush(reply)
}

func (s *Session) onLogin(msg *pb.LoginReq) proto.Message {
	if s.playerID != 0 {
		return &pb.Error{Number: -1, Message: "repeat login"}
	}
	user, err := s.hub.runtime.Users.FindByKey(msg.Password)
	if err != nil {
		return &pb.Error{Number: int32(pb.ErrorCodeInternalError), Message: err.Error()}
	}
	if user == nil {
		return &pb.Error{Number: -2, Message: "Incorrect key"}
	}
	s.playerID = user.ID
	s.hub.registerPlayer(user.ID, s)
	s.hub.runtime.Players.Bind(user.ID, s)
	tunnels := s.hub.runtime.Tunnel.ByPlayer(user.ID)
	reply := &pb.LoginAck{
		PlayerID:                    user.ID,
		SupportsTunnelRuntimeReport: true,
	}
	for _, tunnel := range tunnels {
		reply.TunnelList = append(reply.TunnelList, s.hub.runtimeTunnelPB(tunnel))
	}
	s.hub.broadcastPlayerTunnelStates(user.ID, user.ID)
	return reply
}

func (s *Session) onRegister(msg *pb.RegisterReq) proto.Message {
	code, text, _, err := s.hub.runtime.Players.Add(msg.Username, msg.Password)
	if err != nil {
		return &pb.Error{Number: int32(pb.ErrorCodeInternalError), Message: err.Error()}
	}
	if code != 0 {
		return &pb.Error{Number: code, Message: text}
	}
	return &pb.Success{}
}

func (s *Session) enqueue(serial int32, message proto.Message) error {
	packet, err := codec.Encode(serial, message)
	if err != nil {
		return err
	}
	return s.writeQueue.Push(packet)
}

func (s *Session) String() string {
	var b bytes.Buffer
	b.WriteString("session<")
	b.WriteString(s.conn.RemoteAddr().String())
	b.WriteString(">")
	return b.String()
}

func (h *Hub) runtimeTunnelPB(tunnel model.Tunnel) *pb.Tunnel {
	return &pb.Tunnel{
		Source:           &pb.TunnelPoint{Addr: tunnel.Source},
		Endpoint:         &pb.TunnelPoint{Addr: tunnel.Endpoint},
		ID:               tunnel.ID,
		Enabled:          h.tunnelRuntimeEnabled(tunnel),
		Sender:           tunnel.Sender,
		Receiver:         tunnel.Receiver,
		TunnelType:       int32(tunnel.TunnelType),
		Password:         tunnel.Password,
		Username:         tunnel.Username,
		IsCompressed:     tunnel.IsCompressed,
		EncryptionMethod: tunnel.EncryptionMethod,
		CustomMapping:    tunnel.CustomMapping,
	}
}

func (h *Hub) runtimeTunnelUpdate(tunnel model.Tunnel, isDelete bool) *pb.ModifyTunnelNtf {
	return &pb.ModifyTunnelNtf{
		IsDelete: isDelete,
		Tunnel:   h.runtimeTunnelPB(tunnel),
	}
}

type writeQueue struct {
	mu     sync.Mutex
	cond   *sync.Cond
	items  [][]byte
	closed bool
	max    int
	head   int
	size   int
}

func newWriteQueue() *writeQueue {
	queue := &writeQueue{
		max:   sessionWriteQueueCapacity,
		items: make([][]byte, sessionWriteQueueCapacity),
	}
	queue.cond = sync.NewCond(&queue.mu)
	return queue
}

func (q *writeQueue) Push(data []byte) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return net.ErrClosed
	}
	if q.size >= q.max {
		return errSessionWriteQueueFull
	}
	tail := (q.head + q.size) % q.max
	q.items[tail] = data
	q.size++
	q.cond.Signal()
	return nil
}

func (q *writeQueue) Pop() ([]byte, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for q.size == 0 && !q.closed {
		q.cond.Wait()
	}
	if q.size == 0 {
		return nil, false
	}
	data := q.items[q.head]
	q.items[q.head] = nil
	q.head = (q.head + 1) % q.max
	q.size--
	return data, true
}

func (q *writeQueue) Close() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.closed = true
	for i := range q.items {
		q.items[i] = nil
	}
	q.cond.Broadcast()
}

func writeAllToConn(conn net.Conn, data []byte) error {
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

func clearSessionDeadline(conn net.Conn) {
	if conn == nil {
		return
	}
	_ = conn.SetDeadline(time.Time{})
}

func isExpectedSessionCloseError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, net.ErrClosed) {
		return true
	}
	if ne, ok := err.(net.Error); ok && ne.Timeout() {
		return false
	}
	return false
}

func compactSessionReadCache(buf []byte) []byte {
	if len(buf) == 0 {
		return nil
	}
	if cap(buf) <= sessionReadCacheCompactAt || len(buf)*4 >= cap(buf) {
		return buf
	}
	compacted := make([]byte, len(buf))
	copy(compacted, buf)
	return compacted
}
