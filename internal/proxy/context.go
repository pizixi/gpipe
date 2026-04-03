package proxy

import (
	"fmt"
	"net"
	"sync/atomic"
)

// InletAuthData 保存各类动态代理入口的附加参数。
type InletAuthData struct {
	Username string
	Password string
	Method   string
}

// ContextData 是各代理上下文共享的运行时数据。
type ContextData struct {
	sessionID  atomic.Uint32
	tunnelID   uint32
	mode       TunnelMode
	outputAddr string
	output     OutputFunc
	common     *SessionCommonInfo
	authData   InletAuthData
}

func NewContextData(tunnelID uint32, mode TunnelMode, outputAddr string, output OutputFunc, common *SessionCommonInfo, authData InletAuthData) *ContextData {
	return &ContextData{
		tunnelID:   tunnelID,
		mode:       mode,
		outputAddr: outputAddr,
		output:     output,
		common:     common,
		authData:   authData,
	}
}

func (c *ContextData) SessionID() uint32 {
	return c.sessionID.Load()
}

func (c *ContextData) SetSessionID(id uint32) {
	c.sessionID.Store(id)
}

// ContextHandler 对齐 Rust 的 ProxyContext。
type ContextHandler interface {
	OnStart(data *ContextData, peerAddr net.Addr, writer PeerWriter) error
	OnPeerData(data *ContextData, payload []byte) error
	OnProxyMessage(message ProxyMessage) error
	OnStop(data *ContextData) error
	ReadyForRead() bool
}

// UniversalContext 对应 Rust 的 UniversalProxy。
type UniversalContext struct {
	connected atomic.Bool
	writer    PeerWriter
	data      *ContextData
}

func NewUniversalContext() *UniversalContext {
	return &UniversalContext{}
}

func (c *UniversalContext) OnStart(data *ContextData, peerAddr net.Addr, writer PeerWriter) error {
	c.writer = writer
	c.data = data
	data.output(I2OConnect{
		TunnelID:         data.tunnelID,
		ID:               data.SessionID(),
		TunnelType:       uint8(data.mode),
		IsTCP:            data.mode.IsTCP(),
		IsCompressed:     data.common.IsCompressed,
		Addr:             data.outputAddr,
		EncryptionMethod: string(data.common.Method),
		EncryptionKey:    EncodeKeyToBase64(data.common.Key),
		ClientAddr:       peerAddr.String(),
	})
	return nil
}

func (c *UniversalContext) OnPeerData(data *ContextData, payload []byte) error {
	encoded, err := data.common.EncodeDataAndLimit(payload)
	if err != nil {
		return err
	}
	data.output(I2OSendData{TunnelID: data.tunnelID, ID: data.SessionID(), Data: encoded})
	return nil
}

func (c *UniversalContext) OnProxyMessage(message ProxyMessage) error {
	switch msg := message.(type) {
	case O2IConnect:
		if msg.Success {
			c.connected.Store(true)
			return nil
		}
		if c.writer != nil {
			return c.writer.Close()
		}
	case O2IRecvData:
		if c.writer == nil || c.data == nil {
			return nil
		}
		decoded, err := c.data.common.DecodeData(msg.Data)
		if err != nil {
			return err
		}
		dataLen := len(msg.Data)
		return c.writer.Write(decoded, func() {
			c.data.output(I2ORecvDataResult{TunnelID: c.data.tunnelID, ID: msg.ID, DataLen: uint32(dataLen)})
		})
	case O2IDisconnect:
		if c.writer != nil {
			return c.writer.Close()
		}
	}
	return nil
}

func (c *UniversalContext) OnStop(data *ContextData) error {
	data.output(I2ODisconnect{TunnelID: data.tunnelID, ID: data.SessionID()})
	return nil
}

func (c *UniversalContext) ReadyForRead() bool {
	return c.connected.Load()
}

func parseResolvableAddr(value string) (*net.UDPAddr, error) {
	addr, err := net.ResolveUDPAddr("udp", value)
	if err != nil {
		return nil, fmt.Errorf("解析地址失败: %w", err)
	}
	return addr, nil
}
