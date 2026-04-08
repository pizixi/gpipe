package clientbin

import (
	"encoding/base64"
	"encoding/json"
	"strings"
)

const DefaultShadowsocksMethod = "chacha20-ietf-poly1305"
const placeholderPrefix = "__GPIPE_EMBEDDED_CLIENT_CONFIG_BEGIN__"
const placeholderSuffix = "__GPIPE_EMBEDDED_CLIENT_CONFIG_END__"
const placeholderLength = 4096

// EmbeddedConfig 是打包进专属客户端二进制的连接参数。
type EmbeddedConfig struct {
	Server         string `json:"server"`
	Key            string `json:"key"`
	EnableTLS      bool   `json:"enable_tls"`
	TLSServerName  string `json:"tls_server_name"`
	UseShadowsocks bool   `json:"use_shadowsocks"`
	SSServer       string `json:"ss_server"`
	SSMethod       string `json:"ss_method"`
	SSPassword     string `json:"ss_password"`
}

// PlaceholderValue 返回模板客户端里预埋的固定占位串。
// 服务端会在下载时把整段占位串替换成真实配置。
func PlaceholderValue() string {
	paddingLen := placeholderLength - len(placeholderPrefix) - len(placeholderSuffix)
	if paddingLen < 0 {
		paddingLen = 0
	}
	return placeholderPrefix + strings.Repeat("_", paddingLen) + placeholderSuffix
}

// IsPlaceholderValue 判断当前值是否仍然是模板占位内容。
func IsPlaceholderValue(value string) bool {
	return value == "" || strings.HasPrefix(value, placeholderPrefix)
}

// Normalize 统一清理空白、补默认值，并在未启用 Shadowsocks 时清空相关字段。
func (c EmbeddedConfig) Normalize() EmbeddedConfig {
	c.Server = strings.TrimSpace(c.Server)
	c.Key = strings.TrimSpace(c.Key)
	c.TLSServerName = strings.TrimSpace(c.TLSServerName)
	c.SSServer = strings.TrimSpace(c.SSServer)
	c.SSMethod = strings.TrimSpace(c.SSMethod)
	c.SSPassword = strings.TrimSpace(c.SSPassword)
	if c.SSMethod == "" {
		c.SSMethod = DefaultShadowsocksMethod
	}
	if !c.UseShadowsocks {
		c.SSServer = ""
		c.SSMethod = DefaultShadowsocksMethod
		c.SSPassword = ""
	}
	return c
}

// HasRequiredRuntimeValues 判断是否已经具备直接运行客户端所需的最小参数。
func (c EmbeddedConfig) HasRequiredRuntimeValues() bool {
	return strings.TrimSpace(c.Server) != "" && strings.TrimSpace(c.Key) != ""
}

// Encode 把结构体编码成适合写入二进制的紧凑字符串。
func Encode(config EmbeddedConfig) (string, error) {
	data, err := json.Marshal(config.Normalize())
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}

// Decode 从二进制里的字符串恢复运行配置。
// 如果还是模板占位内容，则返回空配置而不是报错。
func Decode(encoded string) (EmbeddedConfig, error) {
	if IsPlaceholderValue(strings.TrimSpace(encoded)) {
		return EmbeddedConfig{}, nil
	}
	data, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(encoded))
	if err != nil {
		return EmbeddedConfig{}, err
	}
	var config EmbeddedConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return EmbeddedConfig{}, err
	}
	return config.Normalize(), nil
}
