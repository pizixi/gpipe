package manager

import (
	"errors"
	"strings"
	"sync"

	"github.com/pizixi/gpipe/internal/model"
	"github.com/pizixi/gpipe/internal/proxy"
	"github.com/pizixi/gpipe/internal/store"
	"github.com/pizixi/gpipe/internal/util"
)

type TunnelNotifier interface {
	BroadcastTunnel(playerID uint32, tunnel model.Tunnel, isDelete bool)
}

type TunnelManager struct {
	store    *store.TunnelStore
	notifier TunnelNotifier
	runtime  *TunnelRuntimeStore
	players  *PlayerManager
	mu       sync.RWMutex
	tunnels  []model.Tunnel
	lookup   map[uint32]model.Tunnel
}

func NewTunnelManager(tunnelStore *store.TunnelStore, players *PlayerManager, notifier TunnelNotifier, runtime *TunnelRuntimeStore) *TunnelManager {
	return &TunnelManager{
		store:    tunnelStore,
		notifier: notifier,
		runtime:  runtime,
		players:  players,
		lookup:   map[uint32]model.Tunnel{},
	}
}

func (m *TunnelManager) LoadAll() error {
	tunnels, err := m.store.FindAll()
	if err != nil {
		return err
	}
	for i := range tunnels {
		normalizeTunnelFields(&tunnels[i])
	}
	m.mu.Lock()
	m.tunnels = tunnels
	m.rebuildLookupLocked()
	m.mu.Unlock()
	return nil
}

func (m *TunnelManager) All() []model.Tunnel {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]model.Tunnel, len(m.tunnels))
	copy(out, m.tunnels)
	return out
}

func (m *TunnelManager) Query(pageNumber, pageSize int) []model.Tunnel {
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 10
	}
	if pageNumber < 0 {
		pageNumber = 0
	}
	start := pageNumber * pageSize
	m.mu.RLock()
	defer m.mu.RUnlock()
	if start >= len(m.tunnels) {
		return nil
	}
	end := start + pageSize
	if end > len(m.tunnels) {
		end = len(m.tunnels)
	}
	out := make([]model.Tunnel, end-start)
	copy(out, m.tunnels[start:end])
	return out
}

func (m *TunnelManager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.tunnels)
}

func (m *TunnelManager) Get(id uint32) (model.Tunnel, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	tunnel, ok := m.lookup[id]
	return tunnel, ok
}

func (m *TunnelManager) ByPlayer(playerID uint32) []model.Tunnel {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []model.Tunnel
	for _, tunnel := range m.tunnels {
		if tunnel.Sender == playerID || tunnel.Receiver == playerID {
			out = append(out, tunnel)
		}
	}
	return out
}

func (m *TunnelManager) Add(tunnel model.Tunnel) (model.Tunnel, error) {
	normalizeTunnelFields(&tunnel)
	if err := m.validateStatic(tunnel); err != nil {
		return model.Tunnel{}, err
	}
	m.mu.Lock()
	if err := m.validatePortConflictLocked(tunnel, 0); err != nil {
		m.mu.Unlock()
		return model.Tunnel{}, err
	}
	id, err := m.store.Insert(tunnel)
	if err != nil {
		m.mu.Unlock()
		return model.Tunnel{}, err
	}
	tunnel.ID = id
	m.tunnels = append(m.tunnels, tunnel)
	m.lookup[tunnel.ID] = tunnel
	m.mu.Unlock()
	m.notify(tunnel, false)
	return tunnel, nil
}

func (m *TunnelManager) Update(tunnel model.Tunnel) error {
	normalizeTunnelFields(&tunnel)
	if err := m.validateStatic(tunnel); err != nil {
		return err
	}
	m.mu.Lock()
	old, ok := m.lookup[tunnel.ID]
	if !ok {
		m.mu.Unlock()
		return errors.New("tunnel id does not exist")
	}
	if err := m.validatePortConflictLocked(tunnel, tunnel.ID); err != nil {
		m.mu.Unlock()
		return err
	}
	if err := m.store.Update(tunnel); err != nil {
		m.mu.Unlock()
		return err
	}
	// 仅当更新会真正影响 inlet/outlet 是否运行（或归属玩家变更）时才清空 runtime，
	// 避免无谓的"客户端未确认"中间态。客户端在收到 ModifyTunnelNtf 后也会重新上报，
	// 这里的判断只是为了减少 UI 抖动。
	if shouldClearRuntimeOnUpdate(old, tunnel) {
		m.clearRuntime(tunnel.ID)
	}
	for i := range m.tunnels {
		if m.tunnels[i].ID == tunnel.ID {
			m.tunnels[i] = tunnel
			m.lookup[tunnel.ID] = tunnel
			m.mu.Unlock()
			if old.Sender != tunnel.Sender && m.notifier != nil {
				m.notifier.BroadcastTunnel(old.Sender, tunnel, true)
			}
			if old.Receiver != tunnel.Receiver && m.notifier != nil {
				m.notifier.BroadcastTunnel(old.Receiver, tunnel, true)
			}
			m.notify(tunnel, false)
			return nil
		}
	}
	m.mu.Unlock()
	return errors.New("tunnel id does not exist")
}

func (m *TunnelManager) Delete(id uint32) error {
	if err := m.store.Delete(id); err != nil {
		return err
	}
	m.clearRuntime(id)
	m.mu.Lock()
	for i, tunnel := range m.tunnels {
		if tunnel.ID == id {
			m.tunnels = append(m.tunnels[:i], m.tunnels[i+1:]...)
			delete(m.lookup, id)
			m.mu.Unlock()
			m.notify(tunnel, true)
			return nil
		}
	}
	m.mu.Unlock()
	return nil
}

// shouldClearRuntimeOnUpdate 判断隧道更新是否会让现有的 inlet/outlet 失效。
// 如果只是改改备注、密码这类不会触发 inlet/outlet 重建的字段，就不必清空 runtime，
// 避免清空后客户端不重启不上报，导致 Web 端长期显示"客户端未确认"。
func shouldClearRuntimeOnUpdate(oldT, newT model.Tunnel) bool {
	if oldT.Enabled != newT.Enabled {
		return true
	}
	if oldT.Sender != newT.Sender || oldT.Receiver != newT.Receiver {
		return true
	}
	if oldT.InletDescription() != newT.InletDescription() {
		return true
	}
	if oldT.OutletDescription() != newT.OutletDescription() {
		return true
	}
	return false
}

func (m *TunnelManager) clearRuntime(tunnelID uint32) {
	if m.runtime == nil {
		return
	}
	m.runtime.Clear(tunnelID)
}

func (m *TunnelManager) validateStatic(tunnel model.Tunnel) error {
	tunnelType := model.NormalizeTunnelType(tunnel.TunnelType)
	if !tunnelType.Valid() {
		return errors.New("unsupported tunnel type")
	}
	if !util.IsValidTunnelSourceAddress(tunnel.Source) {
		return errors.New("source address format error")
	}
	if tunnelType.RequiresEndpoint() {
		if !util.IsValidTunnelEndpointAddress(tunnel.Endpoint) {
			return errors.New("endpoint address format error")
		}
	}
	if tunnel.Sender != 0 && !m.players.Exists(tunnel.Sender) {
		return errors.New("sender player does not exist")
	}
	if tunnel.Receiver != 0 && !m.players.Exists(tunnel.Receiver) {
		return errors.New("receiver player does not exist")
	}
	if tunnelType == model.TunnelTypeShadowsocks && !proxy.IsSupportedShadowsocksMethod(tunnel.EncryptionMethod) {
		return errors.New("unsupported shadowsocks method")
	}
	return nil
}

func (m *TunnelManager) validatePortConflictLocked(tunnel model.Tunnel, selfID uint32) error {
	port, ok := util.GetTunnelPort(tunnel.Source)
	if ok {
		for _, existing := range m.tunnels {
			if existing.ID == selfID || existing.Receiver != tunnel.Receiver {
				continue
			}
			existingPort, ok := util.GetTunnelPort(existing.Source)
			if !ok {
				continue
			}
			if existingPort == port {
				if tunnelsShareListenerProtocol(existing, tunnel) {
					return errors.New("port already in use")
				}
			}
		}
	}
	return nil
}

func tunnelsShareListenerProtocol(a, b model.Tunnel) bool {
	return tunnelListensTCP(a) && tunnelListensTCP(b) || tunnelListensUDP(a) && tunnelListensUDP(b)
}

func tunnelListensTCP(tunnel model.Tunnel) bool {
	switch model.NormalizeTunnelType(tunnel.TunnelType) {
	case model.TunnelTypeTCP, model.TunnelTypeSOCKS5, model.TunnelTypeHTTP, model.TunnelTypeShadowsocks:
		return true
	default:
		return false
	}
}

func tunnelListensUDP(tunnel model.Tunnel) bool {
	switch model.NormalizeTunnelType(tunnel.TunnelType) {
	case model.TunnelTypeUDP, model.TunnelTypeShadowsocks:
		return true
	default:
		return false
	}
}

func (m *TunnelManager) notify(tunnel model.Tunnel, isDelete bool) {
	if m.notifier == nil {
		return
	}
	m.notifier.BroadcastTunnel(tunnel.Sender, tunnel, isDelete)
	if tunnel.Sender != tunnel.Receiver {
		m.notifier.BroadcastTunnel(tunnel.Receiver, tunnel, isDelete)
	}
}

func normalizeTunnelFields(tunnel *model.Tunnel) {
	// 中文注释：Web 表单和人工录入容易混入空格，入库前统一规整。
	tunnel.Source = strings.TrimSpace(tunnel.Source)
	tunnel.Endpoint = strings.TrimSpace(tunnel.Endpoint)
	tunnel.Description = strings.TrimSpace(tunnel.Description)
	tunnel.Username = strings.TrimSpace(tunnel.Username)
	tunnel.Password = strings.TrimSpace(tunnel.Password)
	tunnel.EncryptionMethod = strings.TrimSpace(tunnel.EncryptionMethod)
	tunnel.TunnelType = model.NormalizeTunnelType(tunnel.TunnelType).WireValue()
	if model.NormalizeTunnelType(tunnel.TunnelType) == model.TunnelTypeShadowsocks {
		tunnel.Username = ""
		if tunnel.EncryptionMethod == "" {
			tunnel.EncryptionMethod = proxy.DefaultShadowsocksMethod
		}
	}
	if tunnel.CustomMapping == nil {
		tunnel.CustomMapping = map[string]string{}
		return
	}
	normalized := make(map[string]string, len(tunnel.CustomMapping))
	for key, value := range tunnel.CustomMapping {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		normalized[key] = strings.TrimSpace(value)
	}
	tunnel.CustomMapping = normalized
}

func (m *TunnelManager) rebuildLookupLocked() {
	m.lookup = make(map[uint32]model.Tunnel, len(m.tunnels))
	for _, tunnel := range m.tunnels {
		m.lookup[tunnel.ID] = tunnel
	}
}
