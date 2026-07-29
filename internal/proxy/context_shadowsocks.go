package proxy

import (
	"fmt"
	"net"
	"sync/atomic"
)

const (
	shadowsocksMaxPendingBytes             = 512 * 1024
	shadowsocksMaxPendingItems             = 4096
	shadowsocksTCPPendingItemOverheadBytes = 24
	shadowsocksUDPPendingItemOverheadBytes = 40
)

type shadowsocksTCPStatus int32

const (
	shadowsocksTCPStatusInit shadowsocksTCPStatus = iota
	shadowsocksTCPStatusConnecting
	shadowsocksTCPStatusRunning
)

type ShadowsocksTCPContext struct {
	status       atomic.Int32
	writer       PeerWriter
	data         *ContextData
	peerAddr     net.Addr
	reader       *shadowsocksStreamReader
	encoder      *shadowsocksStreamWriter
	pending      [][]byte
	pendingBytes int
}

type shadowsocksUDPPendingPacket struct {
	target string
	body   []byte
}

type ShadowsocksUDPContext struct {
	connected    atomic.Bool
	writer       PeerWriter
	data         *ContextData
	peerAddr     net.Addr
	pending      []shadowsocksUDPPendingPacket
	pendingBytes int
}

func NewShadowsocksTCPContext() *ShadowsocksTCPContext {
	ctx := &ShadowsocksTCPContext{}
	ctx.status.Store(int32(shadowsocksTCPStatusInit))
	return ctx
}

func (c *ShadowsocksTCPContext) OnStart(data *ContextData, peerAddr net.Addr, writer PeerWriter) error {
	reader, err := newShadowsocksStreamReader(data.authData.Method, data.authData.Password)
	if err != nil {
		return err
	}
	encoder, err := newShadowsocksStreamWriter(data.authData.Method, data.authData.Password)
	if err != nil {
		return err
	}
	c.data = data
	c.peerAddr = peerAddr
	c.writer = writer
	c.reader = reader
	c.encoder = encoder
	return nil
}

func (c *ShadowsocksTCPContext) OnPeerData(data *ContextData, payload []byte) error {
	chunks, err := c.reader.Feed(payload)
	if err != nil {
		return err
	}
	for _, chunk := range chunks {
		if err := c.handleChunk(chunk); err != nil {
			return err
		}
	}
	return nil
}

func (c *ShadowsocksTCPContext) OnProxyMessage(message ProxyMessage) error {
	switch msg := message.(type) {
	case O2IConnect:
		if !msg.Success {
			if c.writer != nil {
				return c.writer.Close()
			}
			return nil
		}
		if c.status.Load() != int32(shadowsocksTCPStatusConnecting) {
			return nil
		}
		c.status.Store(int32(shadowsocksTCPStatusRunning))
		pending := c.pending
		c.pending = nil
		c.pendingBytes = 0
		for _, payload := range pending {
			if err := c.sendPayload(payload); err != nil {
				return err
			}
		}
	case O2IRecvData:
		decoded, err := c.data.common.DecodeData(msg.Data)
		if err != nil {
			return err
		}
		packet, err := c.encoder.SealData(decoded)
		if err != nil {
			return err
		}
		dataLen := len(msg.Data)
		return c.writer.Write(packet, func() {
			c.data.output(I2ORecvDataResult{TunnelID: c.data.tunnelID, ID: msg.ID, DataLen: uint32(dataLen)})
		})
	case O2IDisconnect:
		if c.writer != nil {
			return c.writer.Close()
		}
	}
	return nil
}

func (c *ShadowsocksTCPContext) OnStop(data *ContextData) error {
	data.output(I2ODisconnect{TunnelID: data.tunnelID, ID: data.SessionID()})
	return nil
}

func (c *ShadowsocksTCPContext) ReadyForRead() bool {
	return true
}

func (c *ShadowsocksTCPContext) handleChunk(chunk []byte) error {
	if len(chunk) == 0 {
		return nil
	}

	switch shadowsocksTCPStatus(c.status.Load()) {
	case shadowsocksTCPStatusInit:
		target, body, err := parseShadowsocksTarget(chunk)
		if err != nil {
			return err
		}
		c.status.Store(int32(shadowsocksTCPStatusConnecting))
		c.data.output(I2OConnect{
			TunnelID:         c.data.tunnelID,
			ID:               c.data.SessionID(),
			TunnelType:       uint8(TunnelModeShadowsocks),
			IsTCP:            true,
			IsCompressed:     c.data.common.IsCompressed,
			Addr:             target.String(),
			EncryptionMethod: string(c.data.common.Method),
			EncryptionKey:    EncodeKeyToBase64(c.data.common.Key),
			ClientAddr:       c.peerAddr.String(),
		})
		if len(body) > 0 {
			return c.bufferPendingPayload(body)
		}
	case shadowsocksTCPStatusConnecting:
		return c.bufferPendingPayload(chunk)
	case shadowsocksTCPStatusRunning:
		return c.sendPayload(chunk)
	}

	return nil
}

func (c *ShadowsocksTCPContext) bufferPendingPayload(payload []byte) error {
	if len(payload) == 0 {
		return nil
	}
	cost := shadowsocksTCPPendingItemOverheadBytes + len(payload)
	if len(c.pending) >= shadowsocksMaxPendingItems || c.pendingBytes+cost > shadowsocksMaxPendingBytes {
		return fmt.Errorf("shadowsocks tcp pending payload too large")
	}
	c.pending = append(c.pending, append([]byte(nil), payload...))
	c.pendingBytes += cost
	return nil
}

func (c *ShadowsocksTCPContext) sendPayload(payload []byte) error {
	encoded, err := c.data.common.EncodeDataAndLimit(payload)
	if err != nil {
		return err
	}
	c.data.output(I2OSendData{TunnelID: c.data.tunnelID, ID: c.data.SessionID(), Data: encoded})
	return nil
}

func NewShadowsocksUDPContext() *ShadowsocksUDPContext {
	return &ShadowsocksUDPContext{}
}

func (c *ShadowsocksUDPContext) OnStart(data *ContextData, peerAddr net.Addr, writer PeerWriter) error {
	if !IsSupportedShadowsocksMethod(data.authData.Method) {
		return fmt.Errorf("unsupported shadowsocks method: %s", data.authData.Method)
	}
	c.data = data
	c.peerAddr = peerAddr
	c.writer = writer
	data.output(I2OConnect{
		TunnelID:         data.tunnelID,
		ID:               data.SessionID(),
		TunnelType:       uint8(TunnelModeShadowsocks),
		IsTCP:            false,
		IsCompressed:     data.common.IsCompressed,
		Addr:             "",
		EncryptionMethod: string(data.common.Method),
		EncryptionKey:    EncodeKeyToBase64(data.common.Key),
		ClientAddr:       peerAddr.String(),
	})
	return nil
}

func (c *ShadowsocksUDPContext) OnPeerData(data *ContextData, payload []byte) error {
	decoded, err := shadowsocksDecryptPacket(data.authData.Method, data.authData.Password, payload)
	if err != nil {
		return err
	}
	target, body, err := parseShadowsocksTarget(decoded)
	if err != nil {
		return err
	}
	if !c.connected.Load() {
		return c.bufferPendingPacket(target.String(), body)
	}
	return c.sendPacket(target.String(), body)
}

func (c *ShadowsocksUDPContext) OnProxyMessage(message ProxyMessage) error {
	switch msg := message.(type) {
	case O2IConnect:
		if !msg.Success {
			return fmt.Errorf("shadowsocks udp connect failed: %s", msg.ErrorInfo)
		}
		c.connected.Store(true)
		pending := c.pending
		c.pending = nil
		c.pendingBytes = 0
		for _, packet := range pending {
			if err := c.sendPacket(packet.target, packet.body); err != nil {
				return err
			}
		}
	case O2IRecvDataFrom:
		decoded, err := c.data.common.DecodeData(msg.Data)
		if err != nil {
			return err
		}
		remote, err := net.ResolveUDPAddr("udp", msg.RemoteAddr)
		if err != nil {
			return err
		}
		target := TargetAddr{IP: remote.IP, Port: uint16(remote.Port)}
		targetBytes, err := target.ToBytes()
		if err != nil {
			return err
		}
		packet, err := shadowsocksEncryptPacket(c.data.authData.Method, c.data.authData.Password, append(targetBytes, decoded...))
		if err != nil {
			return err
		}
		dataLen := len(msg.Data)
		return c.writer.Write(packet, func() {
			c.data.output(I2ORecvDataResult{TunnelID: c.data.tunnelID, ID: msg.ID, DataLen: uint32(dataLen)})
		})
	case O2IDisconnect:
		return nil
	}
	return nil
}

func (c *ShadowsocksUDPContext) bufferPendingPacket(target string, body []byte) error {
	// 除正文外还计入目标字符串和切片/字符串字段的固定开销；包数量另设硬上限，
	// 防止零正文 UDP 包绕过字节预算并无限扩张 pending 切片。
	cost := shadowsocksUDPPendingItemOverheadBytes + len(target) + len(body)
	if len(c.pending) >= shadowsocksMaxPendingItems || c.pendingBytes+cost > shadowsocksMaxPendingBytes {
		return fmt.Errorf("shadowsocks udp pending payload too large")
	}
	c.pending = append(c.pending, shadowsocksUDPPendingPacket{
		target: target,
		body:   append([]byte(nil), body...),
	})
	c.pendingBytes += cost
	return nil
}

func (c *ShadowsocksUDPContext) OnStop(data *ContextData) error {
	data.output(I2ODisconnect{TunnelID: data.tunnelID, ID: data.SessionID()})
	return nil
}

func (c *ShadowsocksUDPContext) ReadyForRead() bool {
	return true
}

func (c *ShadowsocksUDPContext) sendPacket(target string, body []byte) error {
	encoded, err := c.data.common.EncodeDataAndLimit(body)
	if err != nil {
		return err
	}
	c.data.output(I2OSendToData{
		TunnelID:   c.data.tunnelID,
		ID:         c.data.SessionID(),
		Data:       encoded,
		TargetAddr: target,
	})
	return nil
}

func parseShadowsocksTarget(payload []byte) (TargetAddr, []byte, error) {
	if len(payload) == 0 {
		return TargetAddr{}, nil, fmt.Errorf("shadowsocks target address is empty")
	}
	target, size, ok, err := ReadTargetAddr(payload[1:], payload[0])
	if err != nil {
		return TargetAddr{}, nil, err
	}
	if !ok {
		return TargetAddr{}, nil, fmt.Errorf("incomplete shadowsocks target address")
	}
	return target, payload[1+size:], nil
}
