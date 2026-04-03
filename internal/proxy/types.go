package proxy

import "net"

// ProxyMessage 表示入口和出口之间转发的抽象消息。
type ProxyMessage interface {
	SessionID() uint32
}

// OutputFunc 用于把代理消息继续转发到远端或本机另一侧。
type OutputFunc func(ProxyMessage)

type I2OConnect struct {
	TunnelID         uint32
	ID               uint32
	TunnelType       uint8
	IsTCP            bool
	IsCompressed     bool
	Addr             string
	EncryptionMethod string
	EncryptionKey    string
	ClientAddr       string
}

func (m I2OConnect) SessionID() uint32 { return m.ID }

type O2IConnect struct {
	TunnelID  uint32
	ID        uint32
	Success   bool
	ErrorInfo string
}

func (m O2IConnect) SessionID() uint32 { return m.ID }

type I2OSendData struct {
	TunnelID uint32
	ID       uint32
	Data     []byte
}

func (m I2OSendData) SessionID() uint32 { return m.ID }

type I2OSendToData struct {
	TunnelID   uint32
	ID         uint32
	Data       []byte
	TargetAddr string
}

func (m I2OSendToData) SessionID() uint32 { return m.ID }

type O2ISendDataResult struct {
	TunnelID uint32
	ID       uint32
	DataLen  uint32
}

func (m O2ISendDataResult) SessionID() uint32 { return m.ID }

type O2IRecvData struct {
	TunnelID uint32
	ID       uint32
	Data     []byte
}

func (m O2IRecvData) SessionID() uint32 { return m.ID }

type O2IRecvDataFrom struct {
	TunnelID   uint32
	ID         uint32
	Data       []byte
	RemoteAddr string
}

func (m O2IRecvDataFrom) SessionID() uint32 { return m.ID }

type I2ORecvDataResult struct {
	TunnelID uint32
	ID       uint32
	DataLen  uint32
}

func (m I2ORecvDataResult) SessionID() uint32 { return m.ID }

type I2ODisconnect struct {
	TunnelID uint32
	ID       uint32
}

func (m I2ODisconnect) SessionID() uint32 { return m.ID }

type O2IDisconnect struct {
	TunnelID uint32
	ID       uint32
}

func (m O2IDisconnect) SessionID() uint32 { return m.ID }

// TunnelMode 对应 Rust 中入口类型。
type TunnelMode uint32

const (
	TunnelModeTCP TunnelMode = iota
	TunnelModeUDP
	TunnelModeSOCKS5
	TunnelModeHTTP
	TunnelModeShadowsocks
	TunnelModeUnknown TunnelMode = 255
)

func (m TunnelMode) IsTCP() bool {
	return m != TunnelModeUDP
}

func (m TunnelMode) IsSOCKS5() bool {
	return m == TunnelModeSOCKS5
}

func (m TunnelMode) UsesRemoteUDPAddr() bool {
	return m == TunnelModeSOCKS5 || m == TunnelModeShadowsocks
}

// SendResultHook 在底层写入完成后回调，用于释放流控令牌。
type SendResultHook func()

// PeerWriter 抽象本地连接写入行为，兼容 TCP 和 UDP。
type PeerWriter interface {
	Write(data []byte, hook SendResultHook) error
	WriteTo(data []byte, addr net.Addr) error
	Close() error
}
