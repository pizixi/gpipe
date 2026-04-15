package proxy

import (
	"log"
	"strconv"
	"sync"

	"github.com/pizixi/gpipe/internal/model"
	"github.com/pizixi/gpipe/internal/pb"
)

type RemoteSender func(playerID uint32, message any) error

// Manager 对齐 Rust 中 client/proxy_manager 的隧道同步逻辑。
type Manager struct {
	logger       *log.Logger
	selfPlayerID uint32
	sendRemote   RemoteSender

	mu      sync.RWMutex
	tunnels map[uint32]*pb.Tunnel
	inlets  map[uint32]*Inlet
	outlets map[uint32]*Outlet
}

func NewManager(logger *log.Logger, selfPlayerID uint32, sendRemote RemoteSender) *Manager {
	return &Manager{
		logger:       logger,
		selfPlayerID: selfPlayerID,
		sendRemote:   sendRemote,
		tunnels:      map[uint32]*pb.Tunnel{},
		inlets:       map[uint32]*Inlet{},
		outlets:      map[uint32]*Outlet{},
	}
}

func (m *Manager) Close() {
	if m == nil {
		return
	}
	m.SyncTunnels(nil)
}

func (m *Manager) SyncTunnels(tunnels []*pb.Tunnel) {
	want := map[uint32]*pb.Tunnel{}
	for _, tunnel := range tunnels {
		if tunnel != nil {
			want[tunnel.ID] = tunnel
		}
	}

	var stopOutlets []*Outlet
	var stopInlets []*Inlet

	m.mu.Lock()
	m.tunnels = want

	for id, outlet := range m.outlets {
		tunnel := want[id]
		if tunnel == nil || !tunnel.Enabled || tunnel.Sender != m.selfPlayerID || outlet.Description() != outletDescription(tunnel) {
			stopOutlets = append(stopOutlets, outlet)
			delete(m.outlets, id)
		}
	}
	for id, inlet := range m.inlets {
		tunnel := want[id]
		if tunnel == nil || !tunnel.Enabled || tunnel.Receiver != m.selfPlayerID || inlet.Description() != inletDescription(tunnel) {
			stopInlets = append(stopInlets, inlet)
			delete(m.inlets, id)
		}
	}
	m.mu.Unlock()

	for _, outlet := range stopOutlets {
		_ = outlet.Stop()
	}
	for _, inlet := range stopInlets {
		_ = inlet.Stop()
	}

	for _, tunnel := range tunnels {
		if tunnel == nil {
			continue
		}
		m.startOutletIfNeeded(tunnel)
		m.startInletIfNeeded(tunnel)
	}
}

func (m *Manager) HandlePB(message any) {
	msg, tunnelID, ok := BridgeFromPB(message)
	if !ok {
		return
	}
	m.mu.RLock()
	tunnel := m.tunnels[tunnelID]
	inlet := m.inlets[tunnelID]
	outlet := m.outlets[tunnelID]
	m.mu.RUnlock()
	if tunnel == nil {
		return
	}
	playerID := tunnel.Receiver
	if IsI2OMessage(msg) {
		playerID = tunnel.Sender
	}
	if playerID == m.selfPlayerID {
		if IsI2OMessage(msg) {
			if outlet != nil {
				outlet.Input(msg)
			}
		} else if inlet != nil {
			inlet.Input(msg)
		}
	}
}

func (m *Manager) UpdateTunnel(msg *pb.ModifyTunnelNtf) {
	if msg == nil || msg.Tunnel == nil {
		return
	}
	var stopOutlet *Outlet
	var stopInlet *Inlet

	m.mu.Lock()
	if msg.IsDelete {
		delete(m.tunnels, msg.Tunnel.ID)
	} else {
		m.tunnels[msg.Tunnel.ID] = msg.Tunnel
	}
	current := m.tunnels[msg.Tunnel.ID]
	if outlet := m.outlets[msg.Tunnel.ID]; m.shouldStopOutlet(outlet, current) {
		stopOutlet = outlet
		delete(m.outlets, msg.Tunnel.ID)
	}
	if inlet := m.inlets[msg.Tunnel.ID]; m.shouldStopInlet(inlet, current) {
		stopInlet = inlet
		delete(m.inlets, msg.Tunnel.ID)
	}
	m.mu.Unlock()

	if stopOutlet != nil {
		_ = stopOutlet.Stop()
	}
	if stopInlet != nil {
		_ = stopInlet.Stop()
	}
	if msg.IsDelete {
		return
	}
	m.startOutletIfNeeded(msg.Tunnel)
	m.startInletIfNeeded(msg.Tunnel)
}

func (m *Manager) shouldRunOutlet(tunnel *pb.Tunnel) bool {
	return tunnel != nil && tunnel.Enabled && tunnel.Sender == m.selfPlayerID
}

func (m *Manager) shouldRunInlet(tunnel *pb.Tunnel) bool {
	return tunnel != nil && tunnel.Enabled && tunnel.Receiver == m.selfPlayerID
}

func (m *Manager) shouldStopOutlet(outlet *Outlet, tunnel *pb.Tunnel) bool {
	return outlet != nil && (!m.shouldRunOutlet(tunnel) || outlet.Description() != outletDescription(tunnel))
}

func (m *Manager) shouldStopInlet(inlet *Inlet, tunnel *pb.Tunnel) bool {
	return inlet != nil && (!m.shouldRunInlet(tunnel) || inlet.Description() != inletDescription(tunnel))
}

func (m *Manager) startOutletIfNeeded(tunnel *pb.Tunnel) {
	if !m.shouldRunOutlet(tunnel) {
		return
	}
	m.mu.Lock()
	_, ok := m.outlets[tunnel.ID]
	current := m.tunnels[tunnel.ID]
	m.mu.Unlock()
	if ok || current == nil || !m.shouldRunOutlet(current) || outletDescription(current) != outletDescription(tunnel) {
		return
	}

	m.logger.Printf("启动出口 tunnel=%d self=%d", tunnel.ID, m.selfPlayerID)
	outlet := NewOutlet(m.logger, func(msg ProxyMessage) {
		m.routeProxyMessage(tunnel, msg)
	}, outletDescription(tunnel))

	m.mu.Lock()
	current = m.tunnels[tunnel.ID]
	if _, exists := m.outlets[tunnel.ID]; !exists && current != nil && m.shouldRunOutlet(current) && outletDescription(current) == outletDescription(tunnel) {
		m.outlets[tunnel.ID] = outlet
	}
	m.mu.Unlock()
}

func (m *Manager) startInletIfNeeded(tunnel *pb.Tunnel) {
	if !m.shouldRunInlet(tunnel) {
		return
	}
	m.mu.RLock()
	_, ok := m.inlets[tunnel.ID]
	current := m.tunnels[tunnel.ID]
	m.mu.RUnlock()
	if ok || current == nil || !m.shouldRunInlet(current) || inletDescription(current) != inletDescription(tunnel) {
		return
	}

	commonMethod := tunnel.EncryptionMethod
	if tunnelModeFromWireValue(tunnel.TunnelType) == TunnelModeShadowsocks {
		commonMethod = string(EncryptionNone)
	}
	common, err := NewSessionCommonInfoFromName(tunnel.IsCompressed, commonMethod)
	if err != nil {
		m.logger.Printf("创建入口公共配置失败: %v", err)
		return
	}
	source := ""
	if tunnel.Source != nil {
		source = tunnel.Source.Addr
	}
	endpoint := ""
	if tunnel.Endpoint != nil {
		endpoint = tunnel.Endpoint.Addr
	}
	inlet := NewInlet(
		m.logger,
		tunnel.ID,
		tunnelModeFromWireValue(tunnel.TunnelType),
		source,
		endpoint,
		common,
		InletAuthData{
			Username: tunnel.Username,
			Password: tunnel.Password,
			Method:   tunnel.EncryptionMethod,
		},
		func(msg ProxyMessage) {
			m.routeProxyMessage(tunnel, msg)
		},
		inletDescription(tunnel),
	)
	m.logger.Printf("启动入口 tunnel=%d self=%d source=%s endpoint=%s", tunnel.ID, m.selfPlayerID, source, endpoint)
	if err := inlet.Start(); err != nil {
		m.logger.Printf("启动入口失败: %v", err)
		return
	}
	installed := false
	m.mu.Lock()
	current = m.tunnels[tunnel.ID]
	if _, exists := m.inlets[tunnel.ID]; !exists && current != nil && m.shouldRunInlet(current) && inletDescription(current) == inletDescription(tunnel) {
		m.inlets[tunnel.ID] = inlet
		installed = true
	}
	m.mu.Unlock()
	if !installed {
		_ = inlet.Stop()
	}
}

func (m *Manager) routeProxyMessage(tunnel *pb.Tunnel, message ProxyMessage) {
	if tunnel == nil {
		return
	}
	if tunnel.Sender == tunnel.Receiver {
		m.mu.RLock()
		inlet := m.inlets[tunnel.ID]
		outlet := m.outlets[tunnel.ID]
		m.mu.RUnlock()
		if IsI2OMessage(message) {
			if outlet != nil {
				outlet.Input(message)
			}
		} else if inlet != nil {
			inlet.Input(message)
		}
		return
	}
	targetPlayer := tunnel.Receiver
	if IsI2OMessage(message) {
		targetPlayer = tunnel.Sender
	}
	if pbMessage := BridgeToPB(message); pbMessage != nil {
		if err := m.sendRemote(targetPlayer, pbMessage); err != nil {
			if m.handleRemoteSendFailure(tunnel.ID, targetPlayer, message) {
				return
			}
			m.logger.Printf("发送代理消息失败: %v", err)
		}
	}
}

func (m *Manager) handleRemoteSendFailure(tunnelID, targetPlayer uint32, message ProxyMessage) bool {
	// 只有服务端本地隧道需要在远端离线时立即回注失败结果；
	// 普通客户端链路发送失败更接近控制连接异常，不应伪造成业务侧离线事件。
	if m.selfPlayerID != 0 {
		return false
	}

	reply := offlineReplyForRemoteMessage(tunnelID, targetPlayer, message)
	if reply == nil {
		return false
	}

	m.deliverLocalFallback(tunnelID, reply)
	m.logger.Printf("远端玩家离线，已执行本地回退: tunnel=%d player=%d", tunnelID, targetPlayer)
	return true
}

func (m *Manager) deliverLocalFallback(tunnelID uint32, message ProxyMessage) {
	m.mu.RLock()
	inlet := m.inlets[tunnelID]
	outlet := m.outlets[tunnelID]
	m.mu.RUnlock()

	if IsI2OMessage(message) {
		if outlet != nil {
			outlet.Input(message)
		}
		return
	}
	if inlet != nil {
		inlet.Input(message)
	}
}

func offlineReplyForRemoteMessage(tunnelID, targetPlayer uint32, message ProxyMessage) ProxyMessage {
	switch msg := message.(type) {
	case I2OConnect:
		return O2IConnect{
			TunnelID:  tunnelID,
			ID:        msg.ID,
			Success:   false,
			ErrorInfo: "no player " + strconv.FormatUint(uint64(targetPlayer), 10) + " or the player is offline",
		}
	case I2OSendData:
		return O2IDisconnect{TunnelID: tunnelID, ID: msg.ID}
	case I2ORecvDataResult:
		return O2IDisconnect{TunnelID: tunnelID, ID: msg.ID}
	case O2IConnect:
		return I2ODisconnect{TunnelID: tunnelID, ID: msg.ID}
	case O2IRecvData:
		return I2ODisconnect{TunnelID: tunnelID, ID: msg.ID}
	case O2ISendDataResult:
		return I2ODisconnect{TunnelID: tunnelID, ID: msg.ID}
	default:
		return nil
	}
}

func tunnelModeFromWireValue(value int32) TunnelMode {
	switch model.NormalizeTunnelType(uint32(value)) {
	case model.TunnelTypeTCP:
		return TunnelModeTCP
	case model.TunnelTypeUDP:
		return TunnelModeUDP
	case model.TunnelTypeSOCKS5:
		return TunnelModeSOCKS5
	case model.TunnelTypeHTTP:
		return TunnelModeHTTP
	case model.TunnelTypeShadowsocks:
		return TunnelModeShadowsocks
	default:
		return TunnelModeUnknown
	}
}

func inletDescription(tunnel *pb.Tunnel) string {
	t := pbTunnelToModel(tunnel)
	return t.InletDescription()
}

func outletDescription(tunnel *pb.Tunnel) string {
	t := pbTunnelToModel(tunnel)
	return t.OutletDescription()
}

func pbTunnelToModel(tunnel *pb.Tunnel) model.Tunnel {
	source := ""
	if tunnel.Source != nil {
		source = tunnel.Source.Addr
	}
	endpoint := ""
	if tunnel.Endpoint != nil {
		endpoint = tunnel.Endpoint.Addr
	}
	return model.Tunnel{
		ID:               tunnel.ID,
		Source:           source,
		Endpoint:         endpoint,
		Enabled:          tunnel.Enabled,
		Sender:           tunnel.Sender,
		Receiver:         tunnel.Receiver,
		TunnelType:       model.NormalizeTunnelType(uint32(tunnel.TunnelType)).WireValue(),
		Password:         tunnel.Password,
		Username:         tunnel.Username,
		IsCompressed:     tunnel.IsCompressed,
		EncryptionMethod: tunnel.EncryptionMethod,
		CustomMapping:    tunnel.CustomMapping,
	}
}
