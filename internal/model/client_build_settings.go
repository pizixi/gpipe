package model

// ClientBuildSettings 是后台“客户端设置”页面保存的专属客户端下载参数。
type ClientBuildSettings struct {
	Server         string `json:"server"`
	EnableTLS      bool   `json:"enable_tls"`
	TLSServerName  string `json:"tls_server_name"`
	UseShadowsocks bool   `json:"use_shadowsocks"`
	SSServer       string `json:"ss_server"`
	SSMethod       string `json:"ss_method"`
	SSPassword     string `json:"ss_password"`
}
