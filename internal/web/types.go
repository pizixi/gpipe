package web

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
	ID     uint32 `json:"id"`
	Remark string `json:"remark"`
	Key    string `json:"key"`
	Online bool   `json:"online"`
}

type PlayerListResponse struct {
	Players       []PlayerListItem `json:"players"`
	CurPageNumber int              `json:"cur_page_number"`
	TotalCount    int              `json:"total_count"`
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
