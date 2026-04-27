package manager

import (
	"strings"
	"sync"
	"time"
)

// TunnelRuntimeRecord 记录服务端本地代理组件的实际启动状态。
// 当前只有服务端本地入口/出口能可靠记录；客户端本地入口需要客户端协议回报。
type TunnelRuntimeRecord struct {
	TunnelID      uint32
	InletRunning  bool
	OutletRunning bool
	InletError    string
	OutletError   string
	UpdatedAt     time.Time
}

// TunnelRuntimeStore 保存隧道运行态，供 Web 后台展示。
type TunnelRuntimeStore struct {
	mu      sync.RWMutex
	records map[uint32]TunnelRuntimeRecord
}

func NewTunnelRuntimeStore() *TunnelRuntimeStore {
	return &TunnelRuntimeStore{
		records: map[uint32]TunnelRuntimeRecord{},
	}
}

func (s *TunnelRuntimeStore) Get(tunnelID uint32) (TunnelRuntimeRecord, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	record, ok := s.records[tunnelID]
	return record, ok
}

func (s *TunnelRuntimeStore) SetInlet(tunnelID uint32, running bool, errMessage string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	record := s.records[tunnelID]
	record.TunnelID = tunnelID
	record.InletRunning = running
	record.InletError = strings.TrimSpace(errMessage)
	record.UpdatedAt = time.Now().UTC()
	s.records[tunnelID] = record
}

func (s *TunnelRuntimeStore) SetOutlet(tunnelID uint32, running bool, errMessage string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	record := s.records[tunnelID]
	record.TunnelID = tunnelID
	record.OutletRunning = running
	record.OutletError = strings.TrimSpace(errMessage)
	record.UpdatedAt = time.Now().UTC()
	s.records[tunnelID] = record
}

func (s *TunnelRuntimeStore) Clear(tunnelID uint32) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.records, tunnelID)
}
