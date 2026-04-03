package model

import (
	"sort"
	"strconv"
	"strings"
)

type TunnelType uint32

const (
	TunnelTypeTCP TunnelType = iota
	TunnelTypeUDP
	TunnelTypeSOCKS5
	TunnelTypeHTTP
	TunnelTypeShadowsocks
	TunnelTypeUnknown TunnelType = 255
)

// NormalizeTunnelType 统一收敛协议层和运行时里的隧道类型值。
//
// 中文注释：Rust 端虽然 proto 枚举把 3 标成 UNKNOWN，但入口实现实际把 3 当成 HTTP。
// Go 端必须沿用这个历史兼容值，否则 Web 管理面和运行时会出现“可配置但不可运行”的割裂状态。
func NormalizeTunnelType(value uint32) TunnelType {
	switch value {
	case uint32(TunnelTypeTCP):
		return TunnelTypeTCP
	case uint32(TunnelTypeUDP):
		return TunnelTypeUDP
	case uint32(TunnelTypeSOCKS5):
		return TunnelTypeSOCKS5
	case uint32(TunnelTypeHTTP):
		return TunnelTypeHTTP
	case uint32(TunnelTypeShadowsocks):
		return TunnelTypeShadowsocks
	default:
		return TunnelTypeUnknown
	}
}

func (t TunnelType) WireValue() uint32 {
	switch t {
	case TunnelTypeTCP, TunnelTypeUDP, TunnelTypeSOCKS5, TunnelTypeHTTP, TunnelTypeShadowsocks:
		return uint32(t)
	default:
		return uint32(TunnelTypeUnknown)
	}
}

func (t TunnelType) Valid() bool {
	return t != TunnelTypeUnknown
}

func (t TunnelType) RequiresEndpoint() bool {
	return t == TunnelTypeTCP || t == TunnelTypeUDP
}

type Tunnel struct {
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
	CustomMapping    map[string]string `json:"custom_mapping"`
	EncryptionMethod string            `json:"encryption_method"`
}

func (t Tunnel) OutletDescription() string {
	return "id:" + u32(t.ID) + "-sender:" + u32(t.Sender) + "-enabled:" + boolString(t.Enabled)
}

func (t Tunnel) InletDescription() string {
	return "id:" + u32(t.ID) +
		"-source:" + t.Source +
		"-endpoint:" + t.Endpoint +
		"-sender:" + u32(t.Sender) +
		"-receiver:" + u32(t.Receiver) +
		"-tunnel_type:" + u32(t.TunnelType) +
		"-username:" + t.Username +
		"-password:" + t.Password +
		"-enabled:" + boolString(t.Enabled) +
		"-is_compressed:" + boolString(t.IsCompressed) +
		"-encryption_method:" + t.EncryptionMethod +
		"-custom_mapping:[" + stableCustomMappingString(t.CustomMapping) + "]"
}

func u32(v uint32) string {
	return strconv.FormatUint(uint64(v), 10)
}

func boolString(v bool) string {
	if v {
		return "true"
	}
	return "false"
}

func stableCustomMappingString(mapping map[string]string) string {
	if len(mapping) == 0 {
		return ""
	}
	keys := make([]string, 0, len(mapping))
	for key := range mapping {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var builder strings.Builder
	for _, key := range keys {
		builder.WriteString(key)
		builder.WriteByte(':')
		builder.WriteString(mapping[key])
		builder.WriteByte('\n')
	}
	return builder.String()
}
