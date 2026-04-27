package web

import "time"

type LoginReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type GeneralResponse struct {
	Code int32  `json:"code"`
	Msg  string `json:"msg"`
}

type PlayerListRequest struct {
	PageNumber int `json:"page_number"`
	PageSize   int `json:"page_size"`
}

type PlayerListItem struct {
	ID         uint32    `json:"id"`
	Remark     string    `json:"remark"`
	Key        string    `json:"key"`
	CreateTime time.Time `json:"create_time"`
	Online     bool      `json:"online"`
}

type PlayerListResponse struct {
	Players       []PlayerListItem `json:"players"`
	CurPageNumber int              `json:"cur_page_number"`
	TotalCount    int              `json:"total_count"`
}

// ClientBuildSettingsResponse 返回后台“客户端设置”页面的当前值。
type ClientBuildSettingsResponse struct {
	Settings ClientBuildSettingsPayload `json:"settings"`
}

// PlayerClientBuildSettingsRequest 请求指定玩家的客户端生成配置。
type PlayerClientBuildSettingsRequest struct {
	PlayerID uint32 `json:"player_id"`
}

// PlayerClientBuildSettingsResponse 返回玩家专属配置；没有专属配置时返回全局默认配置。
type PlayerClientBuildSettingsResponse struct {
	Settings   ClientBuildSettingsPayload `json:"settings"`
	Customized bool                       `json:"customized"`
}

// ClientBuildSettingsPayload 是前后端之间传输的客户端生成设置结构。
type ClientBuildSettingsPayload struct {
	Server         string `json:"server"`
	EnableTLS      bool   `json:"enable_tls"`
	TLSServerName  string `json:"tls_server_name"`
	UseShadowsocks bool   `json:"use_shadowsocks"`
	SSServer       string `json:"ss_server"`
	SSMethod       string `json:"ss_method"`
	SSPassword     string `json:"ss_password"`
}

// GenerateClientReq 描述一次玩家客户端下载请求。
type GenerateClientReq struct {
	PlayerID uint32                      `json:"player_id"`
	Target   string                      `json:"target"`
	Settings *ClientBuildSettingsPayload `json:"settings,omitempty"`
}

type PlayerRemoveReq struct {
	ID uint32 `json:"id"`
}

type PlayerAddReq struct {
	Remark string `json:"remark"`
	Key    string `json:"key"`
}

type PlayerUpdateReq struct {
	ID     uint32 `json:"id"`
	Remark string `json:"remark"`
	Key    string `json:"key"`
}

type TunnelListRequest struct {
	PageNumber int    `json:"page_number"`
	PageSize   int    `json:"page_size"`
	PlayerID   string `json:"player_id"`
}

type TunnelListItem struct {
	ID               uint32            `json:"id"`
	Source           string            `json:"source"`
	Endpoint         string            `json:"endpoint"`
	Enabled          bool              `json:"enabled"`
	RuntimeStatus    string            `json:"runtime_status"`
	RuntimeRunning   bool              `json:"runtime_running"`
	RuntimeMessage   string            `json:"runtime_message"`
	Sender           uint32            `json:"sender"`
	Receiver         uint32            `json:"receiver"`
	Description      string            `json:"description"`
	TunnelType       uint32            `json:"tunnel_type"`
	Password         string            `json:"password"`
	Username         string            `json:"username"`
	IsCompressed     bool              `json:"is_compressed"`
	EncryptionMethod string            `json:"encryption_method"`
	CustomMapping    map[string]string `json:"custom_mapping"`
}

type TunnelListResponse struct {
	Tunnels       []TunnelListItem `json:"tunnels"`
	CurPageNumber int              `json:"cur_page_number"`
	TotalCount    int              `json:"total_count"`
}

type TunnelRemoveReq struct {
	ID uint32 `json:"id"`
}

type TunnelAddReq struct {
	Source           string            `json:"source"`
	Endpoint         string            `json:"endpoint"`
	Enabled          uint8             `json:"enabled"`
	Sender           uint32            `json:"sender"`
	Receiver         uint32            `json:"receiver"`
	Description      string            `json:"description"`
	TunnelType       uint32            `json:"tunnel_type"`
	Password         string            `json:"password"`
	Username         string            `json:"username"`
	IsCompressed     uint8             `json:"is_compressed"`
	EncryptionMethod string            `json:"encryption_method"`
	CustomMapping    map[string]string `json:"custom_mapping"`
}

type TunnelUpdateReq struct {
	ID               uint32            `json:"id"`
	Source           string            `json:"source"`
	Endpoint         string            `json:"endpoint"`
	Enabled          uint8             `json:"enabled"`
	Sender           uint32            `json:"sender"`
	Receiver         uint32            `json:"receiver"`
	Description      string            `json:"description"`
	TunnelType       uint32            `json:"tunnel_type"`
	Password         string            `json:"password"`
	Username         string            `json:"username"`
	IsCompressed     uint8             `json:"is_compressed"`
	EncryptionMethod string            `json:"encryption_method"`
	CustomMapping    map[string]string `json:"custom_mapping"`
}
