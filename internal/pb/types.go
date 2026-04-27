package pb

type TunnelType int32

const (
	TunnelTypeTCP         TunnelType = 0
	TunnelTypeUDP         TunnelType = 1
	TunnelTypeSOCKS5      TunnelType = 2
	TunnelTypeHTTP        TunnelType = 3
	TunnelTypeShadowsocks TunnelType = 4
	TunnelTypeUnknown     TunnelType = 255
)

type ErrorCode int32

const (
	ErrorCodeNone           ErrorCode = 0
	ErrorCodeInternalError  ErrorCode = -1000
	ErrorCodeInterfaceMiss  ErrorCode = -1001
	ErrorCodePlayerNotLogin ErrorCode = -1002
)

type TunnelPoint struct {
	Addr string `json:"addr"`
}

type Tunnel struct {
	Source           *TunnelPoint      `json:"source,omitempty"`
	Endpoint         *TunnelPoint      `json:"endpoint,omitempty"`
	ID               uint32            `json:"id"`
	Enabled          bool              `json:"enabled"`
	Sender           uint32            `json:"sender"`
	Receiver         uint32            `json:"receiver"`
	TunnelType       int32             `json:"tunnel_type"`
	Password         string            `json:"password"`
	Username         string            `json:"username"`
	IsCompressed     bool              `json:"is_compressed"`
	EncryptionMethod string            `json:"encryption_method"`
	CustomMapping    map[string]string `json:"custom_mapping,omitempty"`
}

type LoginReq struct {
	Version  string `json:"version"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type RegisterReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type ManagementLoginReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginAck struct {
	PlayerID                    uint32    `json:"player_id"`
	TunnelList                  []*Tunnel `json:"tunnel_list,omitempty"`
	SupportsTunnelRuntimeReport bool      `json:"supports_tunnel_runtime_report"`
}

type ManagementLoginAck struct {
	Code int32 `json:"code"`
}

type ModifyTunnelNtf struct {
	IsDelete bool    `json:"is_delete"`
	Tunnel   *Tunnel `json:"tunnel,omitempty"`
}

type TunnelRuntimeReport struct {
	TunnelID  uint32 `json:"tunnel_id"`
	Component string `json:"component"`
	Running   bool   `json:"running"`
	Error     string `json:"error"`
}

type Success struct{}

type Fail struct {
	Number  int32  `json:"number"`
	Message string `json:"message"`
}

type Error struct {
	Number  int32  `json:"number"`
	Message string `json:"message"`
}

type Ping struct {
	Ticks int64 `json:"ticks"`
}

type Pong struct {
	Ticks int64 `json:"ticks"`
}

type I2OConnect struct {
	TunnelID         uint32 `json:"tunnel_id"`
	SessionID        uint32 `json:"session_id"`
	TunnelType       uint32 `json:"tunnel_type"`
	IsTCP            bool   `json:"is_tcp"`
	IsCompressed     bool   `json:"is_compressed"`
	Addr             string `json:"addr"`
	EncryptionMethod string `json:"encryption_method"`
	EncryptionKey    string `json:"encryption_key"`
	ClientAddr       string `json:"client_addr"`
}

type O2IConnect struct {
	TunnelID  uint32 `json:"tunnel_id"`
	SessionID uint32 `json:"session_id"`
	Success   bool   `json:"success"`
	ErrorInfo string `json:"error_info"`
}

type I2OSendData struct {
	TunnelID  uint32 `json:"tunnel_id"`
	SessionID uint32 `json:"session_id"`
	Data      []byte `json:"data"`
}

type O2IRecvData struct {
	TunnelID  uint32 `json:"tunnel_id"`
	SessionID uint32 `json:"session_id"`
	Data      []byte `json:"data"`
}

type I2ODisconnect struct {
	TunnelID  uint32 `json:"tunnel_id"`
	SessionID uint32 `json:"session_id"`
}

type O2IDisconnect struct {
	TunnelID  uint32 `json:"tunnel_id"`
	SessionID uint32 `json:"session_id"`
}

type O2ISendDataResult struct {
	TunnelID  uint32 `json:"tunnel_id"`
	SessionID uint32 `json:"session_id"`
	DataLen   uint32 `json:"data_len"`
}

type I2ORecvDataResult struct {
	TunnelID  uint32 `json:"tunnel_id"`
	SessionID uint32 `json:"session_id"`
	DataLen   uint32 `json:"data_len"`
}

type I2OSendToData struct {
	TunnelID   uint32 `json:"tunnel_id"`
	SessionID  uint32 `json:"session_id"`
	Data       []byte `json:"data"`
	TargetAddr string `json:"target_addr"`
}

type O2IRecvDataFrom struct {
	TunnelID   uint32 `json:"tunnel_id"`
	SessionID  uint32 `json:"session_id"`
	Data       []byte `json:"data"`
	RemoteAddr string `json:"remote_addr"`
}
